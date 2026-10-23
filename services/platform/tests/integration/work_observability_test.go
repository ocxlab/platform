package integration

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/work"
	"ocp.local/platform/internal/core/workmodel"
)

func TestWorkSnapshotScopeAndRecoveryPressure(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	pending := enqueueWork(t, f, store, "pending")
	delayed := enqueueWork(t, f, store, "delayed")
	failed := enqueueWork(t, f, store, "failed")
	_, err := f.admin.Exec(`UPDATE runtime.effects SET created_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, pending)
	must(t, err)
	_, err = f.admin.Exec(`UPDATE runtime.effects SET available_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, delayed)
	must(t, err)
	_, err = f.admin.Exec(`UPDATE runtime.effects SET state='failed',failure_code='exhausted' WHERE id=$1`, failed)
	must(t, err)
	adapter := work.SQLStore{DB: f.app}
	rows, err := adapter.Snapshot(context.Background(), []capability.Domain{capability.Food, capability.Lodging})
	must(t, err)
	if len(rows) != 10 {
		t.Fatalf("missing zero/domain rows: %d", len(rows))
	}
	expected := map[string]int64{"pending": 2, "delayed": 1, "leased": 0, "expired": 0, "failed": 1}
	for _, row := range rows {
		want := expected[row.State]
		if row.Domain == capability.Lodging {
			want = 0
		}
		if row.Count != want {
			t.Fatalf("%s/%s count=%d want=%d", row.Domain, row.State, row.Count, want)
		}
		if row.Domain == capability.Food && row.State == "pending" && row.OldestSeconds < 59 {
			t.Fatal("pending age missing")
		}
	}
	claims, err := adapter.Claim(context.Background(), []capability.Domain{capability.Food}, 1, time.Second)
	must(t, err)
	if len(claims) != 1 {
		t.Fatal("no claim")
	}
	_, err = f.admin.Exec(`UPDATE runtime.effects SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, claims[0].ID)
	must(t, err)
	rows, err = adapter.Snapshot(context.Background(), []capability.Domain{capability.Food})
	must(t, err)
	for _, row := range rows {
		if row.State == "expired" && row.Count != 1 {
			t.Fatal("expired lease not visible")
		}
	}
	for _, arg := range []string{`NULL::text[]`, `ARRAY[]::text[]`, `ARRAY[NULL]::text[]`, `ARRAY['unknown']::text[]`} {
		if _, err := f.app.Exec(`SELECT * FROM runtime.work_snapshot(` + arg + `)`); err == nil {
			t.Fatalf("invalid scope accepted: %s", arg)
		}
	}
	var safe bool
	must(t, f.admin.QueryRow(`SELECT p.proowner='ocp_work_owner'::regrole AND p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']::text[] AND NOT EXISTS(SELECT FROM aclexplode(p.proacl) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE') FROM pg_proc p JOIN pg_namespace n ON p.pronamespace=n.oid WHERE n.nspname='runtime' AND p.proname='work_snapshot'`).Scan(&safe))
	if !safe {
		t.Fatal("unsafe snapshot owner/ACL")
	}
	var visible int
	must(t, f.app.QueryRow(`SELECT count(*) FROM runtime.effects`).Scan(&visible))
	if visible != 0 {
		t.Fatal("snapshot bypassed direct row security")
	}
}

type observedWorkStore struct {
	work.Store
	claims    atomic.Int64
	schedules atomic.Int64
}

func (s *observedWorkStore) Claim(ctx context.Context, domains []capability.Domain, count int, lease time.Duration) ([]workmodel.Effect, error) {
	s.claims.Add(1)
	return s.Store.Claim(ctx, domains, count, lease)
}
func (s *observedWorkStore) NextDelay(ctx context.Context, domains []capability.Domain) (*time.Duration, error) {
	s.schedules.Add(1)
	return s.Store.NextDelay(ctx, domains)
}

// This reproduces the previous Run's cadence over the current Store/Once path;
// it is not an archived-binary or production capacity comparison.
func tickerReference(ctx context.Context, w work.Worker) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, err := w.Once(ctx); err != nil && ctx.Err() == nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func TestWorkSchedulingObservations(t *testing.T) {
	if os.Getenv("OCP_TEST_WORK_OBSERVATIONS") != "true" {
		t.Skip("requires task test:work-observations")
	}
	for _, mode := range []string{"ticker-reference", "notification"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			store := preparedStore(t, f)
			adapter := &observedWorkStore{Store: work.SQLStore{DB: f.app}}
			var completed atomic.Int64
			ctx, cancel := context.WithCancel(context.Background())
			w := work.Worker{Store: adapter, Wake: work.PostgresWake{DSN: workDSN(t, f), Domains: []capability.Domain{capability.Food}}, Domains: []capability.Domain{capability.Food}, Concurrency: 2, Handlers: map[string]work.Handler{"food/catalog.publish": func(context.Context, workmodel.Effect) error { completed.Add(1); return nil }}}
			done := make(chan error, 1)
			go func() {
				if mode == "ticker-reference" {
					done <- tickerReference(ctx, w)
				} else {
					done <- w.Run(ctx)
				}
			}()
			t.Cleanup(func() {
				cancel()
				select {
				case err := <-done:
					must(t, err)
				case <-time.After(6 * time.Second):
					t.Error("observation worker did not stop")
				}
			})
			eventuallyWork(t, func() bool { return adapter.claims.Load() >= 1 })
			// Let the startup scan finish before measuring idle cadence.
			time.Sleep(150 * time.Millisecond)
			c0, s0 := adapter.claims.Load(), adapter.schedules.Load()
			start := time.Now()
			time.Sleep(3200 * time.Millisecond)
			elapsed := time.Since(start)
			idleClaims, idleSchedules := adapter.claims.Load()-c0, adapter.schedules.Load()-s0
			c0, s0 = adapter.claims.Load(), adapter.schedules.Load()
			start = time.Now()
			for i := range 12 {
				enqueueWork(t, f, store, fmt.Sprint("burst-", i))
			}
			eventuallyWork(t, func() bool { return completed.Load() == 12 })
			// Completion includes persisted state, not just handler entry.
			eventuallyWork(t, func() bool {
				var n int
				must(t, f.admin.QueryRow(`SELECT count(*) FROM runtime.effects WHERE state='completed'`).Scan(&n))
				return n == 12
			})
			burst := time.Since(start)
			t.Logf("WORK_OBSERVATION mode=%s idle_seconds=%.3f idle_claims=%d idle_schedules=%d burst_items=12 burst_ms=%.3f burst_claims=%d burst_schedules=%d", mode, elapsed.Seconds(), idleClaims, idleSchedules, float64(burst.Microseconds())/1000, adapter.claims.Load()-c0, adapter.schedules.Load()-s0)
			if mode == "notification" && idleClaims != 0 {
				t.Fatal("notification worker queried during idle window")
			}
			if mode == "ticker-reference" && idleClaims < 2 {
				t.Fatal("reference cadence not exercised")
			}
		})
	}
}
