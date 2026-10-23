package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"ocp.local/platform/internal/database"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/offering"
)

func TestDurableEffectsAndLeaseFencing(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	ctx := context.Background()
	effect := persistence.Effect{MerchantID: store.MerchantID, StoreID: store.ID, Domain: capability.Food, Kind: "catalog.publish", SubjectID: store.ID, Key: "test-generation", Payload: catalogPayload(t)}
	tx, err := database.TenantTransaction(ctx, f.app, store.MerchantID)
	must(t, err)
	_, err = persistence.EnqueueEffect(ctx, tx, effect)
	must(t, err)
	must(t, tx.Rollback())
	rows, err := persistence.ClaimEffects(ctx, f.app, []capability.Domain{capability.Food}, 1, time.Second)
	must(t, err)
	if len(rows) != 0 {
		t.Fatal("rollback retained intent")
	}
	tx, err = database.TenantTransaction(ctx, f.app, store.MerchantID)
	must(t, err)
	id, err := persistence.EnqueueEffect(ctx, tx, effect)
	must(t, err)
	must(t, tx.Commit())
	tx, err = database.TenantTransaction(ctx, f.app, store.MerchantID)
	must(t, err)
	again, err := persistence.EnqueueEffect(ctx, tx, effect)
	must(t, err)
	must(t, tx.Commit())
	if id != again {
		t.Fatal("retry changed effect identity")
	}
	rows, err = persistence.ClaimEffects(ctx, f.app, []capability.Domain{capability.Lodging}, 1, time.Second)
	must(t, err)
	if len(rows) != 0 {
		t.Fatal("wrong domain claimed work")
	}
	rows, err = persistence.ClaimEffects(ctx, f.app, []capability.Domain{capability.Food}, 1, time.Second)
	must(t, err)
	if len(rows) != 1 {
		t.Fatal("missing committed work")
	}
	old := rows[0]
	rows, err = persistence.ClaimEffects(ctx, f.app, []capability.Domain{capability.Food}, 1, time.Second)
	must(t, err)
	if len(rows) != 0 {
		t.Fatal("live lease claimed twice")
	}
	// Simulate a dead worker without sleeping; only the test admin can do this.
	_, err = f.admin.Exec(`UPDATE runtime.effects SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, id)
	must(t, err)
	rows, err = persistence.ClaimEffects(ctx, f.app, []capability.Domain{capability.Food}, 1, 30*time.Second)
	must(t, err)
	current := rows[0]
	if current.ID != old.ID || current.Token == old.Token || current.Generation != old.Generation+1 {
		t.Fatal("lost recovery identity or fence")
	}
	done, err := persistence.FinishEffect(ctx, f.app, old, true, 0)
	must(t, err)
	if done {
		t.Fatal("old lease committed")
	}
	done, err = persistence.FinishEffect(ctx, f.app, current, true, 0)
	must(t, err)
	if !done {
		t.Fatal("live worker cannot finish")
	}
	done, err = persistence.FinishEffect(ctx, f.app, current, true, 0)
	must(t, err)
	if done {
		t.Fatal("duplicate completion")
	}
	var count int
	must(t, f.app.QueryRow(`SELECT count(*) FROM runtime.effects`).Scan(&count))
	if count != 0 {
		t.Fatal("cross-tenant direct read")
	}
	tx, err = database.TenantTransaction(ctx, f.app, uuid.New())
	must(t, err)
	must(t, tx.QueryRow(`SELECT count(*) FROM runtime.effects`).Scan(&count))
	must(t, tx.Rollback())
	if count != 0 {
		t.Fatal("foreign tenant read")
	}
	if _, err = f.app.Exec(`UPDATE runtime.effects SET state='pending'`); err == nil {
		t.Fatal("runtime bypasses fenced updates")
	}
	if _, err = f.app.Exec(`SET ROLE ocp_work_owner`); err == nil {
		t.Fatal("runtime can become work owner")
	}
}

func TestEffectACLConcurrencyRetryAndPublicationRollback(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	store := preparedStore(t, f)
	var functions int
	must(t, f.admin.QueryRow(`SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE n.nspname='runtime' AND p.proname IN ('claim_effects','finish_effect')
 AND p.proowner='ocp_work_owner'::regrole AND p.prosecdef
 AND p.proconfig=ARRAY['search_path=pg_catalog']::text[]
 AND has_function_privilege('ocp_app',p.oid,'EXECUTE')
 AND NOT EXISTS(SELECT FROM aclexplode(p.proacl) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')`).Scan(&functions))
	if functions != 2 {
		t.Fatal("unsafe work function ACL or owner")
	}
	var member bool
	must(t, f.admin.QueryRow(`SELECT pg_has_role('ocp_app','ocp_work_owner','MEMBER')`).Scan(&member))
	if member {
		t.Fatal("worker owner membership")
	}
	// All three records belong to one transaction, including its immutable export.
	tx, err := database.TenantTransaction(ctx, f.app, store.MerchantID)
	must(t, err)
	revision := int64(1)
	_, err = persistence.ChangePublication(ctx, tx, f.principal, store.ID, 0, &revision, nil, "", uuid.New(), func(_ offering.Model, raw json.RawMessage) (json.RawMessage, error) { return raw, nil })
	must(t, err)
	must(t, tx.Rollback())
	var generation, audit, intents int
	must(t, f.admin.QueryRow(`SELECT publication_generation,(SELECT count(*) FROM commerce.publication_audit WHERE store_id=$1),(SELECT count(*) FROM runtime.effects WHERE store_id=$1) FROM commerce.stores WHERE id=$1`, store.ID).Scan(&generation, &audit, &intents))
	if generation != 0 || audit != 0 || intents != 0 {
		t.Fatal("publication rollback split decision and effect")
	}
	// Unknown domain/kind never reaches SQL, while distinct committed work can
	// be claimed concurrently without duplicate deliveries under a live lease.
	for i := 0; i < 4; i++ {
		tx, err := database.TenantTransaction(ctx, f.app, store.MerchantID)
		must(t, err)
		_, err = persistence.EnqueueEffect(ctx, tx, persistence.Effect{MerchantID: store.MerchantID, StoreID: store.ID, Domain: capability.Food, Kind: "catalog.publish", SubjectID: store.ID, Key: fmt.Sprint(i), Payload: catalogPayload(t)})
		must(t, err)
		must(t, tx.Commit())
	}
	var wg sync.WaitGroup
	results := make(chan []persistence.Effect, 2)
	errors := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			rows, err := persistence.ClaimEffects(ctx, f.app, []capability.Domain{capability.Food}, 2, 30*time.Second)
			results <- rows
			errors <- err
		})
	}
	wg.Wait()
	close(results)
	for range 2 {
		must(t, <-errors)
	}
	seen := map[uuid.UUID]bool{}
	var retry persistence.Effect
	for rows := range results {
		for _, e := range rows {
			if seen[e.ID] {
				t.Fatal("duplicate concurrent claim")
			}
			seen[e.ID] = true
			retry = e
		}
	}
	if len(seen) != 4 {
		t.Fatal("concurrent claims lost work")
	}
	done, err := persistence.FinishEffect(ctx, f.app, retry, false, time.Minute)
	must(t, err)
	if !done {
		t.Fatal("retry not persisted")
	}
	rows, err := persistence.ClaimEffects(ctx, f.app, []capability.Domain{capability.Food}, 2, 30*time.Second)
	must(t, err)
	if len(rows) != 0 {
		t.Fatal("backoff ignored")
	}
	_, err = f.admin.Exec(`UPDATE runtime.effects SET available_at=clock_timestamp()-interval '1 second',attempts=19 WHERE id=$1`, retry.ID)
	must(t, err)
	rows, err = persistence.ClaimEffects(ctx, f.app, []capability.Domain{capability.Food}, 1, 30*time.Second)
	must(t, err)
	if len(rows) != 1 || rows[0].Attempts != 20 {
		t.Fatal("retry attempt lost")
	}
	done, err = persistence.FinishEffect(ctx, f.app, rows[0], false, time.Second)
	must(t, err)
	if !done {
		t.Fatal("exhaustion not persisted")
	}
	var state, code string
	must(t, f.admin.QueryRow(`SELECT state,failure_code FROM runtime.effects WHERE id=$1`, retry.ID).Scan(&state, &code))
	if state != "failed" || code != "exhausted" {
		t.Fatal("exhaustion missing")
	}
}

func catalogPayload(t *testing.T) []byte {
	t.Helper()
	revision := int64(1)
	raw, err := json.Marshal(persistence.CatalogIntent{Action: "publish", Visible: true, Generation: 1, Revision: &revision, Model: offering.Model{ID: "ocp.food.menu", Version: "1.0.0"}, DisplayName: "Restaurant", Document: []byte(`{"items":[]}`)})
	must(t, err)
	return raw
}

func TestEffectRejectsUnregisteredAndMalformedWork(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	ctx := context.Background()
	for _, e := range []persistence.Effect{
		{Domain: capability.Food, Kind: "payment.collect", Payload: catalogPayload(t)},
		{Domain: capability.Shopping, Kind: "catalog.publish", Payload: catalogPayload(t)},
		{Domain: capability.Food, Kind: "catalog.publish", Payload: []byte(`{"generation":1}`)},
	} {
		e.MerchantID = store.MerchantID
		e.StoreID = store.ID
		e.SubjectID = store.ID
		e.Key = "invalid"
		tx, err := database.TenantTransaction(ctx, f.app, store.MerchantID)
		must(t, err)
		_, err = persistence.EnqueueEffect(ctx, tx, e)
		must(t, tx.Rollback())
		if err == nil {
			t.Fatal("accepted unsupported work or malformed intent")
		}
	}
}
