package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/persistence"
)

func placedOrderFixture(t *testing.T) (*fixture, persistence.Store, string, string, string, persistence.FoodOrder) {
	t.Helper()
	f, store, private, guest, token, quote := orderFixture(t)
	var order persistence.FoodOrder
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/orders", token, "operations-placement-001", fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, quote.ID), 201), &order))
	return f, store, private, guest, token, order
}

func TestFoodOrderProgressMigrationPreservesLegacyReplay(t *testing.T) {
	f := setupAtVersion(t, 3)
	// Exercise the historical schema without asking current store creation to
	// return display references that did not exist at version 3.
	store := persistence.Store{ID: uuid.New(), MerchantID: f.principal.MerchantID, DisplayName: "Legacy restaurant"}
	_, seedErr := f.admin.Exec(`INSERT INTO commerce.stores(merchant_id,id,display_name) VALUES($1,$2,$3)`, store.MerchantID, store.ID, store.DisplayName)
	must(t, seedErr)
	f.request(t, "PUT", f.merchantPath+"/"+store.ID.String()+"/offering", f.token, "legacy-menu-draft-001", offeringBody(t, 0, menu(t), "ocp.food.menu"), 201)
	private := f.merchantPath + "/" + store.ID.String()
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	guest := strings.Replace(private, "/api/v1/", "/api/v1/guest/", 1)
	f.request(t, "PUT", private+"/publication", f.token, "legacy-menu-publish-001", `{"expected_generation":0,"revision":1}`, 201)
	var session persistence.GuestSession
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &session))
	var quote persistence.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", session.Token, "", selection(1), 201), &quote))
	actor, err := persistence.AuthenticateGuest(context.Background(), f.app, session.Token)
	must(t, err)
	legacy := persistence.FoodOrder{ID: uuid.New(), MerchantID: store.MerchantID, StoreID: store.ID, Contract: "ocp.food.order.v1", Terms: quote, Fulfilment: "pickup", Acceptance: "pending", Payment: "not_started", FulfilmentProgress: "not_started", Resolution: "open", CreatedAt: time.Now().UTC()}
	requestBody := fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, quote.ID)
	var decoded httpapi.PlaceFoodOrderJSONRequestBody
	must(t, json.Unmarshal([]byte(requestBody), &decoded))
	canonical, err := json.Marshal(decoded)
	must(t, err)
	// Seed a real v3 placement transaction: immutable body, event, and historical
	// replay have no operational revision and there is no progress table yet.
	result, err := persistence.Mutate(context.Background(), f.app, actor, "placeFoodOrder", store.ID.String(), "legacy-placement-key-001", canonical, func(tx *sql.Tx) (any, error) {
		body, err := json.Marshal(legacy)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(`INSERT INTO commerce.food_orders(merchant_id,store_id,id,guest_id,quote_id,body,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, store.MerchantID, store.ID, legacy.ID, actor.ID, quote.ID, string(body), legacy.CreatedAt)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(`INSERT INTO commerce.food_order_events(merchant_id,store_id,order_id,sequence,event,actor_id,request_id) VALUES($1,$2,$3,1,'placed',$4,$5)`, store.MerchantID, store.ID, legacy.ID, actor.ID, uuid.New())
		return legacy, err
	})
	must(t, err)
	must(t, persistence.Migrate(context.Background(), f.admin))
	must(t, persistence.Migrate(context.Background(), f.admin))
	var got persistence.FoodOrder
	must(t, json.Unmarshal(f.request(t, "GET", guest+"/food/orders/"+legacy.ID.String(), session.Token, "", "", 200), &got))
	if got.Revision != 1 || len(got.History) != 1 || got.Reference == "" {
		t.Fatal("legacy order was not backfilled")
	}
	if string(f.request(t, "POST", guest+"/food/orders", session.Token, "legacy-placement-key-001", requestBody, 201)) != string(result.Body) {
		t.Fatal("upgrade changed legacy replay")
	}
	f.request(t, "POST", private+"/food/orders/"+legacy.ID.String()+"/decisions", f.token, "legacy-accept-key-001", `{"expected_revision":1,"decision":"accept"}`, 201)
}

func TestFoodOrderDecisionsAndCancellation(t *testing.T) {
	f, store, private, guest, token, order := placedOrderFixture(t)
	path := private + "/food/orders/" + order.ID.String()
	guestPath := guest + "/food/orders/" + order.ID.String()
	decision := `{"expected_revision":1,"decision":"accept"}`
	// Distinct audiences and action scopes, even for a valid credential.
	f.request(t, "POST", path+"/decisions", token, "order-accept-key-001", decision, 401)
	_, readOnly, err := persistence.Provision(context.Background(), f.admin, "Read-only merchant", []string{"order:read"}, time.Now().Add(time.Hour))
	must(t, err)
	f.request(t, "POST", path+"/decisions", readOnly, "order-accept-key-001", decision, 403)
	f.request(t, "GET", path, "", "", "", 401)
	f.request(t, "GET", path, readOnly, "", "", 403)
	_, err = f.admin.Exec(`UPDATE auth.grants SET scopes=ARRAY['order:read'] WHERE id=$1`, f.principal.ID)
	must(t, err)
	f.request(t, "GET", path, f.token, "", "", 200)
	f.request(t, "POST", path+"/decisions", f.token, "read-scope-denied-001", decision, 403)
	_, err = f.admin.Exec(`UPDATE auth.grants SET scopes=ARRAY['store:read','store:write','offering:read','offering:write','order:read','order:manage'] WHERE id=$1`, f.principal.ID)
	must(t, err)
	f.request(t, "POST", path+"/decisions", f.token, "", decision, 400)
	f.request(t, "POST", path+"/decisions", f.token, "invalid-transition-001", `{"expected_revision":1,"decision":"prepare"}`, 400)
	stale := f.request(t, "POST", path+"/decisions", f.token, "stale-revision-key-001", `{"expected_revision":2,"decision":"accept"}`, 409)
	var conflict struct {
		Code    string `json:"code"`
		Current int64  `json:"current_order_revision"`
	}
	must(t, json.Unmarshal(stale, &conflict))
	if conflict.Code != "order_revision_conflict" || conflict.Current != 1 {
		t.Fatalf("missing current order revision: %s", stale)
	}
	accepted := f.request(t, "POST", path+"/decisions", f.token, "order-accept-key-001", decision, 201)
	var got persistence.FoodOrder
	must(t, json.Unmarshal(accepted, &got))
	if got.Revision != 2 || got.Acceptance != "accepted" || got.Payment != "not_started" || got.FulfilmentProgress != "not_started" || len(got.History) != 2 {
		t.Fatalf("invalid acceptance: %s", accepted)
	}
	originalTerms, _ := json.Marshal(order.Terms)
	currentTerms, _ := json.Marshal(got.Terms)
	if string(originalTerms) != string(currentTerms) {
		t.Fatal("acceptance rewrote agreed terms")
	}
	// A different intention cannot override the decision, nor reuse its key.
	f.request(t, "POST", path+"/decisions", f.token, "order-accept-key-001", `{"expected_revision":1,"decision":"reject"}`, 409)
	f.request(t, "POST", path+"/decisions", f.token, "order-reject-key-001", `{"expected_revision":2,"decision":"reject"}`, 409)
	var guestRead persistence.FoodOrder
	must(t, json.Unmarshal(f.request(t, "GET", guestPath, token, "", "", 200), &guestRead))
	if guestRead.Acceptance != "accepted" || len(guestRead.History) != 2 {
		t.Fatal("guest missed merchant decision")
	}
	cancelled := f.request(t, "POST", guestPath+"/cancellation", token, "order-cancel-key-001", `{"expected_revision":2}`, 201)
	must(t, json.Unmarshal(cancelled, &got))
	if got.Revision != 3 || got.Resolution != "cancelled" || len(got.History) != 3 {
		t.Fatalf("invalid cancellation: %s", cancelled)
	}
	if string(f.request(t, "POST", path+"/decisions", f.token, "order-accept-key-001", decision, 201)) != string(accepted) {
		t.Fatal("later progress changed replay")
	}
	f.request(t, "POST", path+"/decisions", f.token, "late-accept-key-001", `{"expected_revision":3,"decision":"accept"}`, 409)
	f.request(t, "POST", guestPath+"/cancellation", token, "new-cancel-key-001", `{"expected_revision":3}`, 409)
	if string(f.request(t, "POST", guestPath+"/cancellation", token, "order-cancel-key-001", `{"expected_revision":2}`, 201)) != string(cancelled) {
		t.Fatal("cancel replay changed")
	}
	// Withdrawal does not cut off purchase management or customer receipts.
	f.request(t, "PUT", private+"/publication", f.token, "operations-withdraw-001", `{"expected_generation":1}`, 201)
	f.request(t, "GET", path, f.token, "", "", 200)
	f.request(t, "GET", guestPath, token, "", "", 200)
	// Admin has its own read-only audience/scope, without mutation impersonation.
	_, adminToken, err := persistence.ProvisionProvider(context.Background(), f.admin, []string{"order:inspect"}, time.Now().Add(time.Hour))
	must(t, err)
	adminPath := strings.Replace(path, "/api/v1/", "/api/v1/admin/", 1)
	f.request(t, "GET", adminPath, f.token, "", "", 401)
	f.request(t, "GET", adminPath, adminToken, "", "", 200)
	f.request(t, "POST", path+"/decisions", adminToken, "admin-accept-key-001", decision, 401)
	_, wrongAdmin, err := persistence.ProvisionProvider(context.Background(), f.admin, []string{"publication:read"}, time.Now().Add(time.Hour))
	must(t, err)
	f.request(t, "GET", adminPath, wrongAdmin, "", "", 403)
	// Restart preserves progress and retry history.
	handler, err := httpapi.NewHandler(f.app)
	must(t, err)
	restarted := httptest.NewServer(handler)
	defer restarted.Close()
	previous := f.server
	f.server = restarted
	defer func() { f.server = previous }()
	f.request(t, "GET", path, f.token, "", "", 200)
	if string(f.request(t, "POST", path+"/decisions", f.token, "order-accept-key-001", decision, 201)) != string(accepted) {
		t.Fatal("restart lost decision replay")
	}
	var actor uuid.UUID
	must(t, f.admin.QueryRow(`SELECT actor_id FROM commerce.food_order_events WHERE order_id=$1 AND sequence=2`, order.ID).Scan(&actor))
	if actor != f.principal.ID {
		t.Fatal("decision actor was not recorded")
	}
	var count int
	must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.food_order_events WHERE merchant_id=$1 AND store_id=$2 AND order_id=$3`, store.MerchantID, store.ID, order.ID).Scan(&count))
	if count != 3 {
		t.Fatal("retry appended duplicate event")
	}
}

func TestFoodOrderRejectionOwnershipAndPages(t *testing.T) {
	f, store, private, guest, token, order := placedOrderFixture(t)
	path := private + "/food/orders/" + order.ID.String()
	guestPath := guest + "/food/orders/" + order.ID.String()
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	var other persistence.GuestSession
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &other))
	f.request(t, "POST", guestPath+"/cancellation", other.Token, "other-guest-cancel-001", `{"expected_revision":1}`, 404)
	f.request(t, "GET", strings.Replace(path, store.ID.String(), uuid.NewString(), 1), f.token, "", "", 404)
	result := f.request(t, "POST", path+"/decisions", f.token, "order-reject-key-002", `{"expected_revision":1,"decision":"reject"}`, 201)
	var got persistence.FoodOrder
	must(t, json.Unmarshal(result, &got))
	if got.Acceptance != "rejected" || got.Resolution != "rejected" {
		t.Fatal("rejection did not close order")
	}
	f.request(t, "POST", guestPath+"/cancellation", token, "rejected-cancel-001", `{"expected_revision":2}`, 409)
	var page persistence.FoodOrderPage
	must(t, json.Unmarshal(f.request(t, "GET", private+"/food/orders?limit=1", f.token, "", "", 200), &page))
	if len(page.Items) != 1 || page.Items[0].Resolution != "rejected" || page.NextAfter != nil {
		t.Fatal("closed order missing from inbox")
	}
	// Exercise a real continuation page, not just an empty cursor result.
	var quote persistence.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", token, "", selection(1), 201), &quote))
	f.request(t, "POST", guest+"/food/orders", token, "second-inbox-order-001", fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, quote.ID), 201)
	must(t, json.Unmarshal(f.request(t, "GET", private+"/food/orders?limit=1", f.token, "", "", 200), &page))
	if len(page.Items) != 1 || page.NextAfter == nil {
		t.Fatal("missing continuation cursor")
	}
	firstID := page.Items[0].ID
	if firstID == order.ID {
		t.Fatal("newest order was not first")
	}
	var next persistence.FoodOrderPage
	must(t, json.Unmarshal(f.request(t, "GET", private+"/food/orders?limit=1&after="+*page.NextAfter, f.token, "", "", 200), &next))
	if len(next.Items) != 1 || next.Items[0].ID == firstID || next.NextAfter != nil {
		t.Fatal("invalid continuation page")
	}
	f.request(t, "GET", private+"/food/orders?limit=101", f.token, "", "", 400)
	f.request(t, "GET", private+"/food/orders?after=invalid", f.token, "", "", 400)
	f.request(t, "GET", strings.Replace(private, store.ID.String(), uuid.NewString(), 1)+"/food/orders", f.token, "", "", 404)
	// Direct tenant-scoped runtime access cannot see other merchants' operations.
	tx, err := persistence.TenantTransaction(context.Background(), f.app, uuid.New())
	must(t, err)
	var count int
	must(t, tx.QueryRow(`SELECT count(*) FROM commerce.food_order_operations`).Scan(&count))
	must(t, tx.Rollback())
	if count != 0 {
		t.Fatal("progress leaked across tenants")
	}
	_, err = f.admin.Exec(`UPDATE commerce.guest_grants SET revoked=true WHERE id=(SELECT guest_id FROM commerce.food_orders WHERE id=$1)`, order.ID)
	must(t, err)
	f.request(t, "POST", guestPath+"/cancellation", token, "revoked-cancel-001", `{"expected_revision":2}`, 401)
}

func TestFoodOrderDecisionAtomicRollback(t *testing.T) {
	f, _, private, _, _, order := placedOrderFixture(t)
	path := private + "/food/orders/" + order.ID.String()
	_, err := f.admin.Exec(`ALTER TABLE commerce.request_replays ADD CONSTRAINT reject_operation_replay CHECK (operation <> 'decideFoodOrder')`)
	must(t, err)
	f.request(t, "POST", path+"/decisions", f.token, "rollback-decision-001", `{"expected_revision":1,"decision":"accept"}`, 503)
	var revision int64
	var count int
	must(t, f.admin.QueryRow(`SELECT revision FROM commerce.food_order_operations WHERE order_id=$1`, order.ID).Scan(&revision))
	must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.food_order_events WHERE order_id=$1`, order.ID).Scan(&count))
	if revision != 1 || count != 1 {
		t.Fatal("failed replay left partial decision")
	}
	_, err = f.admin.Exec(`ALTER TABLE commerce.request_replays DROP CONSTRAINT reject_operation_replay`)
	must(t, err)
	f.request(t, "POST", path+"/decisions", f.token, "rollback-decision-001", `{"expected_revision":1,"decision":"accept"}`, 201)
}

func TestFoodOrderCancellationAtomicRollback(t *testing.T) {
	f, _, private, guest, token, order := placedOrderFixture(t)
	path := guest + "/food/orders/" + order.ID.String() + "/cancellation"
	_, err := f.admin.Exec(`ALTER TABLE commerce.food_order_events ADD CONSTRAINT reject_cancellation_event CHECK (event <> 'cancelled')`)
	must(t, err)
	f.request(t, "POST", path, token, "cancel-atomic-key-001", `{"expected_revision":1}`, 503)
	var current persistence.FoodOrder
	must(t, json.Unmarshal(f.request(t, "GET", private+"/food/orders/"+order.ID.String(), f.token, "", "", 200), &current))
	if current.Revision != 1 || current.Resolution != "open" || len(current.History) != 1 {
		t.Fatal("failed event left partial cancellation")
	}
	_, err = f.admin.Exec(`ALTER TABLE commerce.food_order_events DROP CONSTRAINT reject_cancellation_event`)
	must(t, err)
	f.request(t, "POST", path, token, "cancel-atomic-key-001", `{"expected_revision":1}`, 201)
	f.request(t, "POST", private+"/food/orders/"+order.ID.String()+"/decisions", f.token, "cancel-before-accept-001", `{"expected_revision":2,"decision":"accept"}`, 409)
}

func TestFoodOrderCompetingActions(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(fmt.Sprint("same-key-", same), func(t *testing.T) {
			f, _, private, guest, token, order := placedOrderFixture(t)
			path := private + "/food/orders/" + order.ID.String()
			guestPath := guest + "/food/orders/" + order.ID.String()
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
					url, credential, body, key := path+"/decisions", f.token, `{"expected_revision":1,"decision":"accept"}`, "competing-same-key-001"
					if !same {
						key = fmt.Sprintf("competing-distinct-%02d", i)
						if i%2 == 0 {
							url, credential, body = guestPath+"/cancellation", token, `{"expected_revision":1}`
						}
					}
					req, err := http.NewRequest("POST", f.server.URL+url, strings.NewReader(body))
					if err != nil {
						results <- outcome{err: err}
						return
					}
					req.Header.Set("Authorization", "Bearer "+credential)
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
					if same && first != "" && result.body != first {
						t.Fatal("concurrent replay differed")
					}
					first = result.body
				} else if result.status != 409 {
					t.Fatalf("unexpected %d %s", result.status, result.body)
				}
			}
			expected := 1
			if same {
				expected = 8
			}
			if success != expected {
				t.Fatalf("successful mutations %d", success)
			}
			var revision, count int
			must(t, f.admin.QueryRow(`SELECT revision FROM commerce.food_order_operations WHERE order_id=$1`, order.ID).Scan(&revision))
			must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.food_order_events WHERE order_id=$1`, order.ID).Scan(&count))
			if revision != 2 || count != 2 {
				t.Fatal("competing actions were not serialized")
			}
		})
	}
}
