package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	foodstore "ocp.local/platform/internal/food/persistence"
	"strings"
	"sync"
	"testing"

	"ocp.local/platform/cmd/api/transport/httpapi"
	"ocp.local/platform/internal/core/persistence"
)

func TestDevelopmentPaymentWorkflow(t *testing.T) {
	f, store, private, guest, token, order := placedOrderFixture(t)
	path := guest + "/food/orders/" + order.ID.String()
	pay := path + "/development-payment"
	read := func() foodstore.FoodOrder {
		var got foodstore.FoodOrder
		must(t, json.Unmarshal(f.request(t, "GET", path, token, "", "", 200), &got))
		return got
	}
	act := func(action string, revision int64, key string, want int) []byte {
		return f.request(t, "POST", pay, token, key, fmt.Sprintf(`{"expected_revision":%d,"action":%q}`, revision, action), want)
	}
	if order.Reference == "" || store.Handle == "" {
		t.Fatal("missing human references")
	}
	checkInbox := func(paymentState, provider string, revision int64) {
		t.Helper()
		var page foodstore.FoodOrderPage
		must(t, json.Unmarshal(f.request(t, "GET", private+"/food/orders", f.token, "", "", 200), &page))
		if len(page.Items) != 1 {
			t.Fatalf("unexpected inbox size: %d", len(page.Items))
		}
		item := page.Items[0]
		if item.ID != order.ID || item.Reference != order.Reference || item.Payment != paymentState || item.PaymentProvider != provider || item.Revision != revision {
			t.Fatalf("inbox lost payment state or readable reference: %+v", item)
		}
	}
	checkInbox("not_started", "", 1)
	act("start", 1, "payment-before-accept-001", 409)
	f.request(t, "POST", private+"/food/orders/"+order.ID.String()+"/decisions", f.token, "payment-merchant-accept-001", `{"expected_revision":1,"decision":"accept"}`, 201)
	started := act("start", 2, "payment-start-replay-001", 201)
	act("start", 3, "payment-double-start-001", 409)
	act("decline", 3, "payment-decline-key-001", 201)
	if read().Payment != "failed" {
		t.Fatal("decline not retained")
	}
	act("start", 4, "payment-retry-start-001", 201)
	act("abandon", 5, "payment-abandon-key-001", 201)
	act("start", 6, "payment-third-start-001", 201)
	paid := act("succeed", 7, "payment-success-key-001", 201)
	got := read()
	if got.Payment != "paid" || got.PaymentProvider != "development" || got.Reference != order.Reference || got.Revision != 8 || got.FulfilmentProgress != "not_started" {
		t.Fatalf("unexpected paid state: %+v", got)
	}
	checkInbox("paid", "development", 8)
	var amount int64
	var currency string
	must(t, f.admin.QueryRow(`SELECT amount_minor,currency FROM payment.development_operations WHERE order_id=$1`, order.ID).Scan(&amount, &currency))
	if amount != order.Terms.Total || currency != order.Terms.Currency {
		t.Fatal("payment did not use agreed terms")
	}
	if string(act("start", 2, "payment-start-replay-001", 201)) != string(started) {
		t.Fatal("old start replay changed")
	}
	act("decline", 7, "payment-success-key-001", 409)
	act("succeed", 8, "payment-second-success-001", 409)
	// Process replacement reads persisted state, not memory or a fabricated mock response.
	handler, err := httpapi.NewHandler(f.app, httpapi.WithDevelopmentPayments())
	must(t, err)
	server := httptest.NewServer(handler)
	defer server.Close()
	old := f.server
	f.server = server
	if read().Payment != "paid" {
		t.Fatal("restart lost payment")
	}
	f.server = old
	// The placing guest can cancel before preparation; only this dev adapter
	// can complete a refund synchronously in the same transaction.
	f.request(t, "POST", path+"/cancellation", token, "paid-cancel-order-001", `{"expected_revision":8}`, 201)
	got = read()
	if got.Payment != "refunded" || got.Resolution != "cancelled" || got.Revision != 9 || got.History[len(got.History)-1].Event != "cancelled_refunded" {
		t.Fatalf("missing simulated refund: %+v", got)
	}
	checkInbox("refunded", "development", 9)
	if string(act("succeed", 7, "payment-success-key-001", 201)) != string(paid) {
		t.Fatal("historical success replay changed")
	}
	act("succeed", 9, "payment-late-success-001", 409)
	if read().Payment != "refunded" {
		t.Fatal("late outcome reopened order")
	}
}

func TestDevelopmentPaymentEventFailureRollsBack(t *testing.T) {
	f, _, private, guest, token, order := placedOrderFixture(t)
	path := guest + "/food/orders/" + order.ID.String()
	f.request(t, "POST", private+"/food/orders/"+order.ID.String()+"/decisions", f.token, "payment-rollback-accept-001", `{"expected_revision":1,"decision":"accept"}`, 201)
	// Reject the final event after the payment row and order revision were written.
	_, err := f.admin.Exec(`ALTER TABLE commerce.food_order_events ADD CONSTRAINT reject_payment_event CHECK (event <> 'payment_started')`)
	must(t, err)
	body := `{"expected_revision":2,"action":"start"}`
	f.request(t, "POST", path+"/development-payment", token, "payment-rollback-start-001", body, 503)
	var current foodstore.FoodOrder
	must(t, json.Unmarshal(f.request(t, "GET", path, token, "", "", 200), &current))
	if current.Payment != "not_started" || current.PaymentProvider != "" || current.Revision != 2 || len(current.History) != 2 {
		t.Fatalf("failed event left partial payment or order progress: %+v", current)
	}
	var count int
	must(t, f.admin.QueryRow(`SELECT count(*) FROM payment.development_operations WHERE order_id=$1`, order.ID).Scan(&count))
	if count != 0 {
		t.Fatal("failed event retained payment attempts")
	}
	_, err = f.admin.Exec(`ALTER TABLE commerce.food_order_events DROP CONSTRAINT reject_payment_event`)
	must(t, err)
	first := f.request(t, "POST", path+"/development-payment", token, "payment-rollback-start-001", body, 201)
	if string(first) != string(f.request(t, "POST", path+"/development-payment", token, "payment-rollback-start-001", body, 201)) {
		t.Fatal("successful payment retry changed replay")
	}
	must(t, json.Unmarshal(f.request(t, "GET", path, token, "", "", 200), &current))
	if current.Payment != "awaiting_customer" || current.Revision != 3 || len(current.History) != 3 {
		t.Fatalf("retry did not commit exactly one transition: %+v", current)
	}
	var attempts int
	must(t, f.admin.QueryRow(`SELECT attempts FROM payment.development_operations WHERE order_id=$1`, order.ID).Scan(&attempts))
	if attempts != 1 {
		t.Fatalf("retry duplicated attempts: %d", attempts)
	}
}

func TestDevelopmentPaymentAccessAndDefaultDisabled(t *testing.T) {
	f, _, private, guest, token, order := placedOrderFixture(t)
	path := guest + "/food/orders/" + order.ID.String() + "/development-payment"
	body := `{"expected_revision":1,"action":"start"}`
	f.request(t, "POST", path, "", "payment-no-credential-001", body, 401)
	f.request(t, "POST", path, f.token, "payment-wrong-audience-001", body, 401)
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	var other foodstore.GuestSession
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &other))
	f.request(t, "POST", path, other.Token, "payment-wrong-guest-001", body, 404)
	f.request(t, "POST", path, token, "payment-invalid-amount-001", `{"expected_revision":1,"action":"start","amount_minor":1}`, 400)
	handler, err := httpapi.NewHandler(f.app) // opt-in is not an ambient environment read
	must(t, err)
	server := httptest.NewServer(handler)
	defer server.Close()
	old := f.server
	f.server = server
	f.request(t, "POST", path, token, "payment-mode-disabled-001", body, 503)
	f.server = old
	var count int
	must(t, f.admin.QueryRow(`SELECT count(*) FROM payment.development_operations`).Scan(&count))
	if count != 0 {
		t.Fatal("denied request created payment")
	}
}

func TestPaymentCompletionRacesOrderCancellation(t *testing.T) {
	f, _, private, guest, token, order := placedOrderFixture(t)
	path := guest + "/food/orders/" + order.ID.String()
	f.request(t, "POST", private+"/food/orders/"+order.ID.String()+"/decisions", f.token, "race-payment-accept-001", `{"expected_revision":1,"decision":"accept"}`, 201)
	f.request(t, "POST", path+"/development-payment", token, "race-payment-start-001", `{"expected_revision":2,"action":"start"}`, 201)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, request := range []struct{ suffix, body string }{{"/development-payment", `{"expected_revision":3,"action":"succeed"}`}, {"/cancellation", `{"expected_revision":3}`}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequestWithContext(context.Background(), "POST", f.server.URL+path+request.suffix, strings.NewReader(request.body))
			if err != nil {
				statuses <- 0
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Idempotency-Key", "race-operation-key-001")
			res, err := f.server.Client().Do(req)
			if err != nil {
				statuses <- 0
				return
			}
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			statuses <- res.StatusCode
		}()
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("race results: %v", counts)
	}
	var got foodstore.FoodOrder
	must(t, json.Unmarshal(f.request(t, "GET", path, token, "", "", 200), &got))
	if got.Resolution == "cancelled" && got.Payment != "not_started" {
		t.Fatalf("closed order took money: %+v", got)
	}
	if got.Resolution == "open" && got.Payment != "paid" {
		t.Fatalf("invalid completion: %+v", got)
	}
}

func TestReadableReferencesRemainUniqueAndTenantScoped(t *testing.T) {
	f, store, _, guest, token, order := placedOrderFixture(t)
	var second persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", f.merchantPath, f.token, "same-name-new-store-001", fmt.Sprintf(`{"display_name":%q}`, store.DisplayName), 201), &second))
	if store.Handle == second.Handle || !strings.HasPrefix(store.Handle, "store-") {
		t.Fatal("store references collide")
	}
	var current persistence.Store
	must(t, json.Unmarshal(f.request(t, "GET", f.merchantPath+"/"+store.ID.String(), f.token, "", "", 200), &current))
	if current.Handle != store.Handle {
		t.Fatal("store reference changed")
	}
	var quote foodstore.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", token, "", selection(1), 201), &quote))
	var another foodstore.FoodOrder
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/orders", token, "human-reference-order-002", fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, quote.ID), 201), &another))
	if another.Reference == order.Reference {
		t.Fatal("order references collide")
	}
	f.request(t, "GET", guest+"/food/orders/"+order.Reference, token, "", "", 400)
	var page persistence.PublicStorePage
	must(t, json.Unmarshal(f.request(t, "GET", "/api/v1/public/stores?q="+store.Handle, "", "", "", 200), &page))
	if len(page.Items) != 1 || page.Items[0].ID != store.ID || page.Items[0].Handle != store.Handle {
		t.Fatal("published store handle cannot be found")
	}
}
