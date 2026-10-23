package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	foodstore "ocp.local/platform/internal/food/persistence"
	"strings"
	"sync"
	"testing"
)

func TestPickupProgressRequiresPaymentAndMerchantAuthority(t *testing.T) {
	f, _, private, guest, token, order := placedOrderFixture(t)
	path := private + "/food/orders/" + order.ID.String()
	guestPath := guest + "/food/orders/" + order.ID.String()
	action := func(name string, rev int64, key string, status int) []byte {
		return f.request(t, "POST", path+"/fulfilment", f.token, key, fmt.Sprintf(`{"expected_revision":%d,"action":%q}`, rev, name), status)
	}
	start := `{"expected_revision":1,"action":"start_preparation"}`
	f.request(t, "POST", path+"/fulfilment", "", "pickup-no-auth-001", start, 401)
	f.request(t, "POST", path+"/fulfilment", token, "pickup-guest-denied-001", start, 401)
	action("start_preparation", 1, "pickup-pending-001", 409)
	f.request(t, "POST", path+"/decisions", f.token, "pickup-accept-001", `{"expected_revision":1,"decision":"accept"}`, 201)
	action("start_preparation", 2, "pickup-unpaid-001", 409)
	f.request(t, "POST", guestPath+"/development-payment", token, "pickup-pay-start-001", `{"expected_revision":2,"action":"start"}`, 201)
	action("start_preparation", 3, "pickup-checkout-001", 409)
	f.request(t, "POST", guestPath+"/development-payment", token, "pickup-pay-success-001", `{"expected_revision":3,"action":"succeed"}`, 201)
	action("mark_ready", 4, "pickup-skip-start-001", 409)
	started := action("start_preparation", 4, "pickup-start-001", 201)
	var preparing foodstore.FoodOrder
	must(t, json.Unmarshal(started, &preparing))
	if preparing.FulfilmentProgress != "preparing" || preparing.Revision != 5 {
		t.Fatalf("unexpected preparation: %+v", preparing)
	}
	f.request(t, "POST", guestPath+"/cancellation", token, "pickup-no-auto-cancel-001", `{"expected_revision":5}`, 409)
	action("start_preparation", 5, "pickup-double-start-001", 409)
	action("record_collection", 5, "pickup-skip-ready-001", 409)
	action("mark_ready", 4, "pickup-stale-ready-001", 409)
	ready := action("mark_ready", 5, "pickup-ready-001", 201)
	completed := action("record_collection", 6, "pickup-collected-001", 201)
	if string(action("mark_ready", 5, "pickup-ready-001", 201)) != string(ready) || string(action("record_collection", 6, "pickup-collected-001", 201)) != string(completed) {
		t.Fatal("ready/collection replay changed")
	}
	action("start_preparation", 7, "pickup-no-restart-001", 409)
	action("record_collection", 7, "pickup-no-double-001", 409)
	if string(action("start_preparation", 4, "pickup-start-001", 201)) != string(started) {
		t.Fatal("progress changed historical replay")
	}
	var current foodstore.FoodOrder
	must(t, json.Unmarshal(f.request(t, "GET", guestPath, token, "", "", 200), &current))
	if current.FulfilmentProgress != "collected" || current.Payment != "paid" || current.Revision != 7 || len(current.History) != 7 {
		t.Fatalf("unexpected pickup: %+v", current)
	}
	var response foodstore.FoodOrder
	must(t, json.Unmarshal(completed, &response))
	original, err := json.Marshal(order.Terms)
	must(t, err)
	retained, err := json.Marshal(response.Terms)
	must(t, err)
	if string(original) != string(retained) {
		t.Fatal("fulfilment rewrote agreed terms")
	}
	var page foodstore.FoodOrderPage
	must(t, json.Unmarshal(f.request(t, "GET", private+"/food/orders", f.token, "", "", 200), &page))
	if len(page.Items) != 1 || page.Items[0].FulfilmentProgress != "collected" {
		t.Fatal("inbox lost progress")
	}
}

func TestPreparationAndCancellationSerialize(t *testing.T) {
	f, _, private, guest, token, order := placedOrderFixture(t)
	path := private + "/food/orders/" + order.ID.String()
	guestPath := guest + "/food/orders/" + order.ID.String()
	f.request(t, "POST", path+"/decisions", f.token, "race-pickup-accept-001", `{"expected_revision":1,"decision":"accept"}`, 201)
	f.request(t, "POST", guestPath+"/development-payment", token, "race-pickup-pay-001", `{"expected_revision":2,"action":"start"}`, 201)
	f.request(t, "POST", guestPath+"/development-payment", token, "race-pickup-paid-001", `{"expected_revision":3,"action":"succeed"}`, 201)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, operation := range []struct{ path, token, body string }{
		{path + "/fulfilment", f.token, `{"expected_revision":4,"action":"start_preparation"}`},
		{guestPath + "/cancellation", token, `{"expected_revision":4}`},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequestWithContext(context.Background(), "POST", f.server.URL+operation.path, strings.NewReader(operation.body))
			if err != nil {
				statuses <- 0
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+operation.token)
			req.Header.Set("Idempotency-Key", "race-pickup-operation-001")
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
	must(t, json.Unmarshal(f.request(t, "GET", guestPath, token, "", "", 200), &got))
	if got.Revision != 5 {
		t.Fatalf("race advanced more than once: %+v", got)
	}
	if got.Resolution == "cancelled" {
		if got.Payment != "refunded" || got.FulfilmentProgress != "not_started" {
			t.Fatalf("cancelled order started work: %+v", got)
		}
	} else if got.Payment != "paid" || got.FulfilmentProgress != "preparing" {
		t.Fatalf("invalid preparation: %+v", got)
	}
}

func TestCancelledOrderCannotStartPreparation(t *testing.T) {
	f, _, private, guest, token, order := placedOrderFixture(t)
	path := private + "/food/orders/" + order.ID.String()
	guestPath := guest + "/food/orders/" + order.ID.String()
	f.request(t, "POST", path+"/decisions", f.token, "closed-pickup-accept-001", `{"expected_revision":1,"decision":"accept"}`, 201)
	f.request(t, "POST", guestPath+"/development-payment", token, "closed-pickup-pay-001", `{"expected_revision":2,"action":"start"}`, 201)
	f.request(t, "POST", guestPath+"/development-payment", token, "closed-pickup-paid-001", `{"expected_revision":3,"action":"succeed"}`, 201)
	f.request(t, "POST", guestPath+"/cancellation", token, "closed-pickup-cancel-001", `{"expected_revision":4}`, 201)
	f.request(t, "POST", path+"/fulfilment", f.token, "closed-pickup-start-001", `{"expected_revision":5,"action":"start_preparation"}`, 409)
}
