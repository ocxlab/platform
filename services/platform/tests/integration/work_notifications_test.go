package integration

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/core/work"
	"ocp.local/platform/internal/core/workmodel"
	"ocp.local/platform/internal/infra/database"
)

func workDSN(t *testing.T, f *fixture) string {
	t.Helper()
	dsn, err := url.Parse(os.Getenv("OCP_TEST_DATABASE_URL"))
	must(t, err)
	var name string
	must(t, f.admin.QueryRow(`SELECT current_database()`).Scan(&name))
	dsn.Path = "/" + name
	dsn.User = url.UserPassword("ocp_app", "ocp_app_local_only")
	return dsn.String()
}
func enqueueWork(t *testing.T, f *fixture, store persistence.Store, key string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	tx, err := database.TenantTransaction(ctx, f.app, store.MerchantID)
	must(t, err)
	id, err := persistence.EnqueueEffect(ctx, tx, persistence.Effect{MerchantID: store.MerchantID, StoreID: store.ID, SubjectID: store.ID, Domain: capability.Food, Kind: "catalog.publish", Key: key, Payload: catalogPayload(t)})
	must(t, err)
	must(t, tx.Commit())
	return id
}
func eventuallyWork(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("work condition timed out")
}
func startWork(t *testing.T, f *fixture, handler work.Handler) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	w := work.Worker{Store: work.SQLStore{DB: f.app}, Wake: work.PostgresWake{DSN: workDSN(t, f), Domains: []capability.Domain{capability.Food}}, Domains: []capability.Domain{capability.Food}, Concurrency: 2, SafetyInterval: time.Hour, Handlers: map[string]work.Handler{"food/catalog.publish": handler}}
	go func() { done <- w.Run(ctx) }()
	stop := func() {
		cancel()
		select {
		case err := <-done:
			must(t, err)
		case <-time.After(6 * time.Second):
			t.Error("worker shutdown timed out")
		}
	}
	t.Cleanup(stop)
	return cancel
}
func TestWorkNotificationsCommitRollbackAndScheduleACL(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	ctx := context.Background()
	listener, err := pgx.Connect(ctx, workDSN(t, f))
	must(t, err)
	defer listener.Close(ctx)
	_, err = listener.Exec(ctx, "LISTEN ocp_work_changed")
	must(t, err)
	tx, err := database.TenantTransaction(ctx, f.app, store.MerchantID)
	must(t, err)
	_, err = persistence.EnqueueEffect(ctx, tx, persistence.Effect{MerchantID: store.MerchantID, StoreID: store.ID, SubjectID: store.ID, Domain: capability.Food, Kind: "catalog.publish", Key: "rollback-notification", Payload: catalogPayload(t)})
	must(t, err)
	must(t, tx.Rollback())
	short, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	_, err = listener.WaitForNotification(short)
	cancel()
	if err == nil {
		t.Fatal("rolled-back work emitted a hint")
	}
	enqueueWork(t, f, store, "commit-notification")
	short, cancel = context.WithTimeout(ctx, 2*time.Second)
	n, err := listener.WaitForNotification(short)
	cancel()
	must(t, err)
	if n.Channel != "ocp_work_changed" || n.Payload != "food" {
		t.Fatalf("non-minimal wake payload: %+v", n)
	}
	adapter := work.SQLStore{DB: f.app}
	due, err := adapter.NextDelay(ctx, []capability.Domain{capability.Food})
	must(t, err)
	if due == nil || *due > time.Second {
		t.Fatal("committed work missing from schedule")
	}
	due, err = adapter.NextDelay(ctx, []capability.Domain{capability.Lodging})
	must(t, err)
	if due != nil {
		t.Fatal("foreign domain schedule leaked")
	}
	for _, arg := range []string{`ARRAY[]::text[]`, `ARRAY['unknown']::text[]`, `ARRAY[NULL]::text[]`, `NULL::text[]`} {
		if _, err := f.app.Exec(`SELECT runtime.next_effect_delay(` + arg + `)`); err == nil {
			t.Fatalf("invalid schedule scope accepted: %s", arg)
		}
	}
	var safe bool
	must(t, f.admin.QueryRow(`SELECT p.proowner='ocp_work_owner'::regrole AND p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']::text[] AND NOT EXISTS(SELECT FROM aclexplode(p.proacl) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE') FROM pg_proc p JOIN pg_namespace n ON p.pronamespace=n.oid WHERE n.nspname='runtime' AND p.proname='next_effect_delay'`).Scan(&safe))
	if !safe {
		t.Fatal("unsafe schedule function owner/ACL")
	}
}
func TestWorkRetryAndDeadLeaseTimersWithoutNewInsert(t *testing.T) {
	for _, recoverLease := range []bool{false, true} {
		t.Run(fmt.Sprint("lease=", recoverLease), func(t *testing.T) {
			f := setup(t)
			store := preparedStore(t, f)
			id := enqueueWork(t, f, store, "before-listener")
			if recoverLease {
				rows, err := persistence.ClaimEffects(context.Background(), f.app, []capability.Domain{capability.Food}, 1, time.Second)
				must(t, err)
				if len(rows) != 1 {
					t.Fatal("no abandoned lease")
				}
			}
			var calls atomic.Int32
			startWork(t, f, func(ctx context.Context, e workmodel.Effect) error {
				if e.ID != id {
					return fmt.Errorf("identity changed")
				}
				attempt := calls.Add(1)
				if !recoverLease && attempt == 1 {
					return fmt.Errorf("transient receiver failure")
				}
				return nil
			})
			eventuallyWork(t, func() bool {
				var state string
				must(t, f.admin.QueryRow(`SELECT state FROM runtime.effects WHERE id=$1`, id).Scan(&state))
				return state == "completed"
			})
			want := int32(2)
			if recoverLease {
				want = 1
			}
			if calls.Load() != want {
				t.Fatalf("calls=%d want=%d", calls.Load(), want)
			}
		})
	}
}
func TestWorkReconnectAndTwoWorkers(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	var calls atomic.Int32
	handler := func(context.Context, workmodel.Effect) error { calls.Add(1); return nil }
	startWork(t, f, handler)
	startWork(t, f, handler)
	eventuallyWork(t, func() bool {
		var n int
		must(t, f.admin.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND application_name='ocp-work-wake' AND state='idle'`).Scan(&n))
		return n == 2
	})
	// Drop both direct listeners; insert while their reconnect backoff is active.
	_, err := f.admin.Exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=current_database() AND application_name='ocp-work-wake'`)
	must(t, err)
	for i := range 21 {
		enqueueWork(t, f, store, fmt.Sprint("reconnect-", i))
	}
	eventuallyWork(t, func() bool {
		var n int
		must(t, f.admin.QueryRow(`SELECT count(*) FROM runtime.effects WHERE state='completed'`).Scan(&n))
		return n == 21
	})
	if calls.Load() != 21 {
		t.Fatalf("live leases delivered more than once: %d", calls.Load())
	}
}
func TestWorkUpgradeKeepsExistingIntent(t *testing.T) {
	f := setupAtVersion(t, 18)
	store := preparedStore(t, f)
	id := enqueueWork(t, f, store, "pre-upgrade")
	must(t, database.Migrate(context.Background(), f.admin))
	var calls atomic.Int32
	startWork(t, f, func(context.Context, workmodel.Effect) error { calls.Add(1); return nil })
	eventuallyWork(t, func() bool {
		var state string
		must(t, f.admin.QueryRow(`SELECT state FROM runtime.effects WHERE id=$1`, id).Scan(&state))
		return state == "completed"
	})
	if calls.Load() != 1 {
		t.Fatal("upgrade lost existing intent")
	}
}

// countedWorkStore observes scheduler calls without changing their SQL behavior.
type countedWorkStore struct {
	work.Store
	schedules atomic.Int32
}

func (s *countedWorkStore) NextDelay(ctx context.Context, domains []capability.Domain) (*time.Duration, error) {
	delay, err := s.Store.NextDelay(ctx, domains)
	s.schedules.Add(1)
	return delay, err
}
func TestWorkEarlierDeadlineInterruptsWaitAndExhaustedLeaseTerminates(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	old := enqueueWork(t, f, store, "delayed")
	_, err := f.admin.Exec(`UPDATE runtime.effects SET available_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, old)
	must(t, err)
	adapter := &countedWorkStore{Store: work.SQLStore{DB: f.app}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	w := work.Worker{Store: adapter, Wake: work.PostgresWake{DSN: workDSN(t, f), Domains: []capability.Domain{capability.Food}}, Domains: []capability.Domain{capability.Food}, Concurrency: 2, SafetyInterval: time.Hour, Handlers: map[string]work.Handler{"food/catalog.publish": func(context.Context, workmodel.Effect) error { return nil }}}
	go func() { done <- w.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			must(t, err)
		case <-time.After(6 * time.Second):
			t.Error("worker did not stop")
		}
	})
	eventuallyWork(t, func() bool { return adapter.schedules.Load() >= 1 })
	id := enqueueWork(t, f, store, "earlier")
	eventuallyWork(t, func() bool {
		var state string
		must(t, f.admin.QueryRow(`SELECT state FROM runtime.effects WHERE id=$1`, id).Scan(&state))
		return state == "completed"
	})
	// No insert at the expiry instant. The final abandoned lease must fail,
	// not spin on the overdue row or be delivered for a twenty-first attempt.
	_, err = f.admin.Exec(`UPDATE runtime.effects SET state='leased',attempts=20,lease_token=$2,lease_generation=20,lease_until=clock_timestamp()+interval '300 milliseconds' WHERE id=$1`, old, uuid.New())
	must(t, err)
	eventuallyWork(t, func() bool {
		var state, code string
		must(t, f.admin.QueryRow(`SELECT state,coalesce(failure_code,'') FROM runtime.effects WHERE id=$1`, old).Scan(&state, &code))
		return state == "failed" && code == "exhausted"
	})
}

type silentWorkWake struct{}

func (silentWorkWake) Run(ctx context.Context, _ chan<- work.WakeSignal) error {
	<-ctx.Done()
	return nil
}

func TestWorkSafetyRecoversWithoutAnyListenerHint(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	id := enqueueWork(t, f, store, "missed-hint")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	w := work.Worker{Store: work.SQLStore{DB: f.app}, Wake: silentWorkWake{}, SafetyInterval: 100 * time.Millisecond, Domains: []capability.Domain{capability.Food}, Concurrency: 2, Handlers: map[string]work.Handler{"food/catalog.publish": func(context.Context, workmodel.Effect) error { return nil }}}
	go func() { done <- w.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			must(t, err)
		case <-time.After(6 * time.Second):
			t.Error("safety worker did not stop")
		}
	})
	eventuallyWork(t, func() bool {
		var state string
		must(t, f.admin.QueryRow(`SELECT state FROM runtime.effects WHERE id=$1`, id).Scan(&state))
		return state == "completed"
	})
}

func TestWorkGracefulShutdownPersistsUnfinishedRetry(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	id := enqueueWork(t, f, store, "cancel-attempt")
	started := make(chan struct{})
	stop := startWork(t, f, func(ctx context.Context, e workmodel.Effect) error { close(started); <-ctx.Done(); return ctx.Err() })
	select {
	case <-started:
	case <-time.After(6 * time.Second):
		t.Fatal("handler never started")
	}
	stop()
	eventuallyWork(t, func() bool {
		var state string
		var leased bool
		must(t, f.admin.QueryRow(`SELECT state,lease_token IS NOT NULL FROM runtime.effects WHERE id=$1`, id).Scan(&state, &leased))
		return state == "pending" && !leased
	})
	var attempts int
	must(t, f.admin.QueryRow(`SELECT attempts FROM runtime.effects WHERE id=$1`, id).Scan(&attempts))
	if attempts != 1 {
		t.Fatal("shutdown claimed again")
	}
}

func TestWorkListenerIgnoresUnknownPayload(t *testing.T) {
	f := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake := work.PostgresWake{DSN: workDSN(t, f), Domains: []capability.Domain{capability.Food}}
	signals := make(chan work.WakeSignal, 1)
	done := make(chan error, 1)
	go func() { done <- wake.Run(ctx, signals) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			must(t, err)
		case <-time.After(3 * time.Second):
			t.Error("listener did not stop")
		}
	})
	select {
	case signal := <-signals:
		if signal != work.Listening {
			t.Fatal("missing ready signal")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not start")
	}
	_, err := f.admin.Exec(`SELECT pg_notify('ocp_work_changed','private-payload')`)
	must(t, err)
	select {
	case <-signals:
		t.Fatal("unknown payload woke worker")
	case <-time.After(150 * time.Millisecond):
	}
	_, err = f.admin.Exec(`SELECT pg_notify('ocp_work_changed','food')`)
	must(t, err)
	select {
	case signal := <-signals:
		if signal != work.Changed {
			t.Fatal("wrong wake")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("valid domain ignored")
	}
}
