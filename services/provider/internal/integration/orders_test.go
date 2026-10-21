package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/persistence"
)

func TestGuestOrderPlacement(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	private := f.merchantPath + "/" + store.ID.String()
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	guest := strings.Replace(private, "/api/v1/", "/api/v1/guest/", 1)
	f.request(t, "PUT", private+"/publication", f.token, "publish-order-menu-001", `{"expected_generation":0,"revision":1}`, 201)
	var session, other persistence.GuestSession
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &session))
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &other))
	var quote persistence.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", session.Token, "", selection(1), 201), &quote))
	body := fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, quote.ID)
	path := guest + "/food/orders"
	f.request(t, "POST", path, "", "place-food-order-001", body, 401)
	f.request(t, "POST", path, f.token, "place-food-order-001", body, 401)
	f.request(t, "POST", path, other.Token, "place-food-order-001", body, 404)
	first := f.request(t, "POST", path, session.Token, "place-food-order-001", body, 201)
	var order persistence.FoodOrder
	must(t, json.Unmarshal(first, &order))
	byReference := path + "/by-reference/" + order.Reference
	var resolved persistence.FoodOrder
	must(t, json.Unmarshal(f.request(t, "GET", byReference, session.Token, "", "", 200), &resolved))
	if resolved.ID != order.ID {
		t.Fatal("reference resolved a different order")
	}
	f.request(t, "GET", byReference, "", "", "", 401)
	f.request(t, "GET", byReference, other.Token, "", "", 404)
	f.request(t, "GET", strings.Replace(byReference, store.ID.String(), store.MerchantID.String(), 1), session.Token, "", "", 403)
	f.request(t, "GET", path+"/by-reference/O-000000", session.Token, "", "", 404)
	f.request(t, "GET", path+"/by-reference/O-9999999999999999999", session.Token, "", "", 404)
	if order.Terms.Total != 1400 || order.Acceptance != "pending" || order.Payment != "not_started" || order.FulfilmentProgress != "not_started" {
		t.Fatalf("incorrect placement: %+v", order)
	}
	f.request(t, "GET", path+"/"+order.ID.String(), other.Token, "", "", 404)
	f.request(t, "GET", guest+"/food/quotes/"+quote.ID.String(), other.Token, "", "", 404)
	f.request(t, "GET", path+"/"+order.ID.String(), session.Token, "", "", 200)
	f.request(t, "POST", path, session.Token, "place-food-order-002", body, 409)
	f.request(t, "POST", path, session.Token, "place-food-order-001", strings.Replace(body, quote.ID.String(), order.ID.String(), 1), 409)
	_, err := f.admin.Exec(`UPDATE commerce.food_quotes SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, quote.ID)
	must(t, err)
	if !bytes.Equal(first, f.request(t, "POST", path, session.Token, "place-food-order-001", body, 201)) {
		t.Fatal("successful retry changed after expiry")
	}
	var count int
	must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.food_orders`).Scan(&count))
	if count != 1 {
		t.Fatalf("duplicate orders: %d", count)
	}
	must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.food_order_events`).Scan(&count))
	if count != 1 {
		t.Fatalf("duplicate placement events: %d", count)
	}
	// Existing access is independent of publication, quote cleanup, and draft edits.
	f.request(t, "PUT", private+"/publication", f.token, "withdraw-order-menu-001", `{"expected_generation":1}`, 201)
	_, err = f.admin.Exec(`DELETE FROM commerce.food_quotes WHERE id=$1`, quote.ID)
	must(t, err)
	f.request(t, "GET", path+"/"+order.ID.String(), session.Token, "", "", 200)
	if !bytes.Equal(first, f.request(t, "POST", path, session.Token, "place-food-order-001", body, 201)) {
		t.Fatal("replay depends on ephemeral quote or publication")
	}
	f.request(t, "GET", byReference, session.Token, "", "", 200)
	_, err = f.admin.Exec(`UPDATE commerce.guest_grants SET revoked=true WHERE merchant_id=$1`, store.MerchantID)
	must(t, err)
	f.request(t, "GET", path+"/"+order.ID.String(), session.Token, "", "", 401)
	f.request(t, "GET", byReference, session.Token, "", "", 401)
	f.request(t, "POST", path, session.Token, "place-food-order-001", body, 401)
}

func orderFixture(t *testing.T) (*fixture, persistence.Store, string, string, string, persistence.FoodQuote) {
	t.Helper()
	f := setup(t)
	store := preparedStore(t, f)
	private := f.merchantPath + "/" + store.ID.String()
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	guest := strings.Replace(private, "/api/v1/", "/api/v1/guest/", 1)
	f.request(t, "PUT", private+"/publication", f.token, "publish-guard-menu-001", `{"expected_generation":0,"revision":1}`, 201)
	var s persistence.GuestSession
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &s))
	var quote persistence.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", s.Token, "", selection(1), 201), &quote))
	return f, store, private, guest, s.Token, quote
}

func TestOrderPriceAndAvailabilityGuards(t *testing.T) {
	for _, scenario := range []string{"price_only", "unavailable", "expired", "public_preview", "wrong_store", "wrong_merchant"} {
		t.Run(scenario, func(t *testing.T) {
			f, store, private, guest, token, q := orderFixture(t)
			want := 201
			switch scenario {
			case "price_only", "unavailable":
				doc := menu(t)
				item := doc["items"].([]any)[0].(map[string]any)
				if scenario == "price_only" {
					item["variants"].([]any)[0].(map[string]any)["price_minor"] = 999
				} else {
					item["available"] = false
					want = 422
				}
				f.request(t, "PUT", private+"/offering", f.token, "new-guard-draft-001", offeringBody(t, 1, doc, "ocp.food.menu"), 201)
				f.request(t, "PUT", private+"/publication", f.token, "new-guard-publish-001", `{"expected_generation":1,"revision":2}`, 201)
			case "expired":
				_, err := f.admin.Exec(`UPDATE commerce.food_quotes SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, q.ID)
				must(t, err)
				want = 409
			case "public_preview":
				public := strings.Replace(guest, "/guest/", "/public/", 1)
				must(t, json.Unmarshal(f.request(t, "POST", public+"/food/quotes", "", "", selection(1), 201), &q))
				want = 404
			case "wrong_store":
				guest = strings.Replace(guest, store.ID.String(), store.MerchantID.String(), 1)
				want = 403
			case "wrong_merchant":
				guest = strings.Replace(guest, store.MerchantID.String(), store.ID.String(), 1)
				want = 403
			}
			body := fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, q.ID)
			response := f.request(t, "POST", guest+"/food/orders", token, "guard-place-order-001", body, want)
			if want == 201 {
				var order persistence.FoodOrder
				must(t, json.Unmarshal(response, &order))
				if order.Terms.Total != 1400 {
					t.Fatal("issued price was rewritten")
				}
			}
		})
	}
}

func TestConcurrentOrderPlacement(t *testing.T) {
	for _, sameKey := range []bool{true, false} {
		t.Run(fmt.Sprint(sameKey), func(t *testing.T) {
			f, _, _, guest, token, q := orderFixture(t)
			body := fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, q.ID)
			type outcome struct {
				status int
				body   string
				err    error
			}
			results := make(chan outcome, 8)
			var wait sync.WaitGroup
			for i := 0; i < 8; i++ {
				wait.Add(1)
				go func(i int) {
					defer wait.Done()
					key := "concurrent-placement-same"
					if !sameKey {
						key = fmt.Sprintf("concurrent-placement-%04d", i)
					}
					req, err := http.NewRequest("POST", f.server.URL+guest+"/food/orders", strings.NewReader(body))
					if err != nil {
						results <- outcome{err: err}
						return
					}
					req.Header.Set("Authorization", "Bearer "+token)
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Idempotency-Key", key)
					response, err := f.server.Client().Do(req)
					if err != nil {
						results <- outcome{err: err}
						return
					}
					defer response.Body.Close()
					data, err := io.ReadAll(response.Body)
					results <- outcome{response.StatusCode, string(data), err}
				}(i)
			}
			wait.Wait()
			close(results)
			success := 0
			first := ""
			for result := range results {
				must(t, result.err)
				if result.status == 201 {
					success++
					if first == "" {
						first = result.body
					} else if sameKey && first != result.body {
						t.Fatal("replays differed")
					}
				} else if result.status != 409 {
					t.Fatalf("unexpected %d: %s", result.status, result.body)
				}
			}
			expected := 1
			if sameKey {
				expected = 8
			}
			if success != expected {
				t.Fatalf("successful responses %d", success)
			}
			var count int
			must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.food_orders`).Scan(&count))
			if count != 1 {
				t.Fatal("duplicate order")
			}
			// A rebuilt handler can recover the original persistent result.
			handler, err := httpapi.NewHandler(f.app)
			must(t, err)
			restarted := httptest.NewServer(handler)
			defer restarted.Close()
			if sameKey {
				old := f.server
				f.server = restarted
				if string(f.request(t, "POST", guest+"/food/orders", token, "concurrent-placement-same", body, 201)) != first {
					t.Fatal("restart changed replay")
				}
				f.server = old
			}
		})
	}
}

func TestOrderStorageAuthorityAndRollback(t *testing.T) {
	f, store, _, guest, token, q := orderFixture(t)
	// Runtime cannot rewrite immutable orders/events, or grant expiry/digests.
	var permitted bool
	for _, privilege := range []string{"UPDATE", "DELETE"} {
		must(t, f.admin.QueryRow(`SELECT has_table_privilege('ocp_app','commerce.food_orders',$1) OR has_table_privilege('ocp_app','commerce.food_order_events',$1)`, privilege).Scan(&permitted))
		if permitted {
			t.Fatal("runtime can rewrite order history")
		}
	}
	must(t, f.admin.QueryRow(`SELECT has_column_privilege('ocp_app','commerce.guest_grants','expires_at','UPDATE')`).Scan(&permitted))
	if permitted {
		t.Fatal("runtime can extend guest grants")
	}
	var hardened bool
	must(t, f.admin.QueryRow(`SELECT p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog'] AND NOT EXISTS(SELECT FROM aclexplode(p.proacl) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE') FROM pg_proc p WHERE p.oid='auth.lookup_guest_grant(bytea)'::regprocedure`).Scan(&hardened))
	if !hardened {
		t.Fatal("guest lookup privilege boundary changed")
	}
	tx, err := persistence.TenantTransaction(context.Background(), f.app, store.ID)
	must(t, err)
	var visible int
	must(t, tx.QueryRow(`SELECT count(*) FROM commerce.food_quotes`).Scan(&visible))
	must(t, tx.Rollback())
	if visible != 0 {
		t.Fatal("cross-tenant quote visible")
	}
	// Failure of the replay insert rolls back both the order and placement event.
	_, err = f.admin.Exec(`ALTER TABLE commerce.guest_request_replays ADD CONSTRAINT reject_checkpoint_replay CHECK (false)`)
	must(t, err)
	body := fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, q.ID)
	f.request(t, "POST", guest+"/food/orders", token, "atomic-placement-001", body, 503)
	var count int
	must(t, f.admin.QueryRow(`SELECT (SELECT count(*) FROM commerce.food_orders)+(SELECT count(*) FROM commerce.food_order_events)`).Scan(&count))
	if count != 0 {
		t.Fatal("failed placement left partial records")
	}
	_, err = f.admin.Exec(`ALTER TABLE commerce.guest_request_replays DROP CONSTRAINT reject_checkpoint_replay`)
	must(t, err)
	f.request(t, "POST", guest+"/food/orders", token, "atomic-placement-001", body, 201)
}

func TestAbandonedGuestCleanupPreservesOrderActors(t *testing.T) {
	f, store, private, guest, token, q := orderFixture(t)
	body := fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, q.ID)
	f.request(t, "POST", guest+"/food/orders", token, "retained-actor-order-001", body, 201)
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201)
	_, err := f.admin.Exec(`UPDATE commerce.guest_grants SET expires_at=clock_timestamp()-interval '1 day' WHERE merchant_id=$1`, store.MerchantID)
	must(t, err)
	_, err = f.admin.Exec(`UPDATE commerce.food_quotes SET expires_at=clock_timestamp()-interval '1 day' WHERE merchant_id=$1`, store.MerchantID)
	must(t, err)
	f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201)
	var count int
	must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.guest_grants WHERE merchant_id=$1`, store.MerchantID).Scan(&count))
	if count != 2 {
		t.Fatalf("expected historical actor and new guest, got %d", count)
	}
	must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.food_orders`).Scan(&count))
	if count != 1 {
		t.Fatal("cleanup erased order")
	}
}
