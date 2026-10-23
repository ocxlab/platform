package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	foodstore "ocp.local/platform/internal/food/persistence"
	"ocp.local/platform/internal/infra/database"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"ocp.local/platform/internal/core/persistence"
)

func preparedStore(t *testing.T, f *fixture) persistence.Store {
	t.Helper()
	var store persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", f.merchantPath, f.token, "create-public-store-001", `{"display_name":"Neighbourhood Döner"}`, 201), &store))
	f.request(t, "PUT", f.merchantPath+"/"+store.ID.String()+"/offering", f.token, "draft-public-menu-001", offeringBody(t, 0, menu(t), "ocp.food.menu"), 201)
	return store
}
func selection(generation int64) string {
	return fmt.Sprintf(`{"publication_generation":%d,"lines":[{"item_id":"doener","variant_id":"regular","quantity":2,"options":[{"group_id":"sauce","option_id":"garlic"}]}]}`, generation)
}

func TestPublicationJourney(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	ctx := context.Background()
	private := f.merchantPath + "/" + store.ID.String()
	public := "/api/v1/public/merchants/" + store.MerchantID.String() + "/stores/" + store.ID.String()
	admin := strings.Replace(private, "/api/v1/", "/api/v1/admin/", 1) + "/publication"
	provider, token, err := persistence.ProvisionProvider(ctx, f.admin, []string{"publication:read", "publication:manage"}, time.Now().Add(time.Hour))
	must(t, err)
	f.request(t, "GET", public+"/offering", "", "", "", 404)
	f.request(t, "GET", admin, f.token, "", "", 401)
	f.request(t, "GET", private, token, "", "", 401)
	f.request(t, "GET", admin, "", "", "", 401)
	f.request(t, "GET", admin, token, "", "", 200)
	body := `{"expected_generation":0,"revision":1}`
	first := f.request(t, "PUT", private+"/publication", f.token, "publish-first-menu-001", body, 201)
	if !bytes.Equal(first, f.request(t, "PUT", private+"/publication", f.token, "publish-first-menu-001", body, 201)) {
		t.Fatal("publication replay changed")
	}
	f.request(t, "PUT", private+"/publication", f.token, "publish-first-menu-001", `{"expected_generation":1}`, 409)
	projection := f.request(t, "GET", public+"/offering", "", "", "", 200)
	for _, field := range []string{"token_hash", "reason", "actor_id", "created_at", "state"} {
		if bytes.Contains(projection, []byte(`"`+field+`"`)) {
			t.Fatalf("private field %s leaked", field)
		}
	}
	var q foodstore.FoodQuote
	quoteBody := f.request(t, "POST", public+"/food/quotes", "", "", selection(1), 201)
	must(t, json.Unmarshal(quoteBody, &q))
	if q.Total != 1400 || q.Generation != 1 || q.ExpiresAt.Sub(q.CreatedAt) != 10*time.Minute {
		t.Fatalf("wrong quote: %+v", q)
	}
	var saved []byte
	must(t, f.admin.QueryRow(`SELECT body FROM commerce.food_quotes WHERE merchant_id=$1 AND id=$2`, store.MerchantID, q.ID).Scan(&saved))
	var stored foodstore.FoodQuote
	must(t, json.Unmarshal(saved, &stored))
	if stored.Total != q.Total {
		t.Fatal("quote not retained")
	}
	// Quotes are explicitly non-idempotent and create no purchase/payment effects.
	var again foodstore.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", public+"/food/quotes", "", "", selection(1), 201), &again))
	if again.ID == q.ID {
		t.Fatal("quote should be a new snapshot")
	}
	for _, bad := range []string{
		strings.Replace(selection(1), `"options":[{"group_id":"sauce","option_id":"garlic"}]`, `"options":[]`, 1),
		strings.Replace(selection(1), `"regular"`, `"missing"`, 1),
		strings.Replace(selection(1), `"garlic"`, `"missing"`, 1),
		strings.Replace(selection(1), `{"group_id":"sauce","option_id":"garlic"}`, `{"group_id":"sauce","option_id":"garlic"},{"group_id":"sauce","option_id":"garlic"}`, 1),
	} {
		f.request(t, "POST", public+"/food/quotes", "", "", bad, 422)
	}
	f.request(t, "POST", public+"/food/quotes", "", "", strings.Replace(selection(1), `"quantity":2`, `"quantity":0`, 1), 400)
	f.request(t, "POST", public+"/food/quotes", "", "", strings.Replace(selection(1), `"quantity":2`, `"quantity":2,"total_minor":1`, 1), 400)
	f.request(t, "POST", public+"/food/quotes", "", "", selection(2), 409)
	// Draft 2 must not change the public pointer, prices, or retained quote.
	doc := menu(t)
	doc["items"].([]any)[0].(map[string]any)["variants"].([]any)[0].(map[string]any)["price_minor"] = 900
	f.request(t, "PUT", private+"/offering", f.token, "draft-price-update-001", offeringBody(t, 1, doc, "ocp.food.menu"), 201)
	if !bytes.Equal(projection, f.request(t, "GET", public+"/offering", "", "", "", 200)) {
		t.Fatal("draft changed public offering")
	}
	f.request(t, "PUT", private+"/publication", f.token, "publish-price-update-001", `{"expected_generation":1,"revision":2}`, 201)
	f.request(t, "POST", public+"/food/quotes", "", "", selection(1), 409)
	var revised foodstore.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", public+"/food/quotes", "", "", selection(2), 201), &revised))
	if revised.Total != 1800 {
		t.Fatal("new quote did not use new price")
	}
	// Scope denial, suspension, merchant edits under suspension, restore then withdrawal.
	_, readToken, err := persistence.ProvisionProvider(ctx, f.admin, []string{"publication:read"}, time.Now().Add(time.Hour))
	must(t, err)
	suspend := `{"expected_generation":2,"suspended":true,"reason":"Founder moderation review"}`
	f.request(t, "PUT", admin, readToken, "suspend-menu-read-denied", suspend, 403)
	suspended := f.request(t, "PUT", admin, token, "suspend-menu-founder-001", suspend, 201)
	if !bytes.Equal(suspended, f.request(t, "PUT", admin, token, "suspend-menu-founder-001", suspend, 201)) {
		t.Fatal("admin retry changed result")
	}
	f.request(t, "GET", public+"/offering", "", "", "", 404)
	f.request(t, "POST", public+"/food/quotes", "", "", selection(3), 404)
	f.request(t, "PUT", private+"/publication", f.token, "merchant-republish-suspended", `{"expected_generation":3,"revision":1}`, 201)
	var hiddenIntent persistence.CatalogIntent
	var hiddenPayload []byte
	must(t, f.admin.QueryRow(`SELECT payload FROM runtime.effects WHERE store_id=$1 AND effect_key='4'`, store.ID).Scan(&hiddenPayload))
	must(t, json.Unmarshal(hiddenPayload, &hiddenIntent))
	if hiddenIntent.Action != "publish" || hiddenIntent.Visible || string(hiddenIntent.Document) != "null" {
		t.Fatal("suspended merchant publication exposed a catalog")
	}
	f.request(t, "GET", public+"/offering", "", "", "", 404)
	f.request(t, "PUT", admin, token, "restore-menu-founder-001", `{"expected_generation":4,"suspended":false,"reason":"Review complete"}`, 201)
	f.request(t, "GET", public+"/offering", "", "", "", 200)
	f.request(t, "PUT", private+"/publication", f.token, "withdraw-menu-merchant-001", `{"expected_generation":5}`, 201)
	f.request(t, "GET", public+"/offering", "", "", "", 404)
	var audits int
	must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.publication_audit WHERE merchant_id=$1 AND store_id=$2`, store.MerchantID, store.ID).Scan(&audits))
	if audits != 6 {
		t.Fatalf("audit count %d", audits)
	}
	var actor string
	must(t, f.admin.QueryRow(`SELECT actor_id::text FROM commerce.publication_audit WHERE merchant_id=$1 AND store_id=$2 AND generation=3`, store.MerchantID, store.ID).Scan(&actor))
	if actor != provider.ID.String() {
		t.Fatal("missing attributable actor")
	}
	var correlated bool
	must(t, f.admin.QueryRow(`SELECT request_id IS NOT NULL FROM commerce.publication_audit WHERE merchant_id=$1 AND store_id=$2 AND generation=3`, store.MerchantID, store.ID).Scan(&correlated))
	if !correlated {
		t.Fatal("audit lacks request correlation")
	}
	_, err = f.app.Exec(`UPDATE commerce.publication_audit SET reason='changed'`)
	if err == nil {
		t.Fatal("audit mutable by runtime")
	}
	_, err = f.app.Exec(`SELECT * FROM auth.provider_grants`)
	if err == nil {
		t.Fatal("runtime can read provider credentials")
	}
	_, err = f.admin.Exec(`UPDATE auth.provider_grants SET revoked=true WHERE id=$1`, provider.ID)
	must(t, err)
	f.request(t, "PUT", admin, token, "suspend-menu-founder-001", suspend, 401)
	var oldTotal int64
	must(t, f.admin.QueryRow(`SELECT (body->>'total_minor')::bigint FROM commerce.food_quotes WHERE merchant_id=$1 AND id=$2`, store.MerchantID, q.ID).Scan(&oldTotal))
	if oldTotal != 1400 {
		t.Fatal("old terms changed")
	}
}

func TestQuoteSeesConcurrentSuspension(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	private := f.merchantPath + "/" + store.ID.String() + "/publication"
	f.request(t, "PUT", private, f.token, "quote-lock-publish-001", `{"expected_generation":0,"revision":1}`, 201)
	tx, err := f.admin.BeginTx(context.Background(), nil)
	must(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`SELECT id FROM commerce.stores WHERE merchant_id=$1 AND id=$2 FOR UPDATE`, store.MerchantID, store.ID)
	must(t, err)
	result := make(chan int, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		req, _ := http.NewRequestWithContext(ctx, "POST", f.server.URL+"/api/v1/public/merchants/"+store.MerchantID.String()+"/stores/"+store.ID.String()+"/food/quotes", strings.NewReader(selection(1)))
		req.Header.Set("Content-Type", "application/json")
		response, err := f.server.Client().Do(req)
		if err != nil {
			result <- 0
			return
		}
		response.Body.Close()
		result <- response.StatusCode
	}()
	// Observe the actual SQL lock wait rather than assuming goroutine timing.
	waiting := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		must(t, f.admin.QueryRow(`SELECT EXISTS(SELECT FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%public_document%')`).Scan(&waiting))
		if waiting {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("quote never waited on publication lock")
	}
	_, err = tx.Exec(`UPDATE commerce.stores SET publication_suspended=true,publication_generation=publication_generation+1 WHERE merchant_id=$1 AND id=$2`, store.MerchantID, store.ID)
	must(t, err)
	must(t, tx.Commit())
	if status := <-result; status != 404 {
		t.Fatalf("quote escaped concurrent suspension: %d", status)
	}
	var count int
	must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.food_quotes WHERE merchant_id=$1`, store.MerchantID).Scan(&count))
	if count != 0 {
		t.Fatal("suspended quote persisted")
	}
}

func TestQuoteQuotaAndExpiredCleanup(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	private := f.merchantPath + "/" + store.ID.String() + "/publication"
	f.request(t, "PUT", private, f.token, "quote-quota-publish-001", `{"expected_generation":0,"revision":1}`, 201)
	public := "/api/v1/public/merchants/" + store.MerchantID.String() + "/stores/" + store.ID.String() + "/food/quotes"
	_, err := f.admin.Exec(`INSERT INTO commerce.food_quotes(merchant_id,store_id,id,body,expires_at) SELECT $1,$2,gen_random_uuid(),'{}'::jsonb,clock_timestamp()+interval '10 minutes' FROM generate_series(1,1000)`, store.MerchantID, store.ID)
	must(t, err)
	f.request(t, "POST", public, "", "", selection(1), 429)
	req, err := http.NewRequest("POST", f.server.URL+public, strings.NewReader(selection(1)))
	must(t, err)
	req.Header.Set("Content-Type", "application/json")
	res, err := f.server.Client().Do(req)
	must(t, err)
	res.Body.Close()
	if res.StatusCode != 429 || res.Header.Get("Retry-After") != "60" {
		t.Fatalf("capacity response lacks retry guidance: %d %q", res.StatusCode, res.Header.Get("Retry-After"))
	}
	_, err = f.admin.Exec(`UPDATE commerce.food_quotes SET expires_at=clock_timestamp()-interval '1 minute' WHERE merchant_id=$1`, store.MerchantID)
	must(t, err)
	f.request(t, "POST", public, "", "", selection(1), 201)
	var count int
	must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.food_quotes WHERE merchant_id=$1`, store.MerchantID).Scan(&count))
	if count != 1 {
		t.Fatalf("expired snapshots not pruned: %d", count)
	}
	tx, err := database.TenantTransaction(context.Background(), f.app, store.MerchantID)
	must(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE commerce.food_quotes SET body='{}'`)
	if err == nil {
		t.Fatal("retained quote mutable by runtime")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatal("expected SQL privilege denial")
	}
}

func TestPublicationConcurrentChanges(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	path := f.merchantPath + "/" + store.ID.String() + "/publication"
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, err := http.NewRequest("PUT", f.server.URL+path, strings.NewReader(`{"expected_generation":0,"revision":1}`))
			if err != nil {
				statuses <- 0
				return
			}
			req.Header.Set("Authorization", "Bearer "+f.token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", fmt.Sprintf("concurrent-publish-%02d", i))
			res, err := f.server.Client().Do(req)
			if err != nil {
				statuses <- 0
				return
			}
			res.Body.Close()
			statuses <- res.StatusCode
		}(i)
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("publication lost update: %v", counts)
	}
}
