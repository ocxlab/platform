package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	foodstore "ocp.local/platform/internal/food/persistence"
	"strings"
	"testing"
	"time"

	"ocp.local/platform/internal/core/persistence"
)

func TestOrderingControls(t *testing.T) {
	f, store, private, guest, token, quote := orderFixture(t)
	settings := private + "/food/ordering"
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	var state foodstore.FoodOrdering
	must(t, json.Unmarshal(f.request(t, "GET", settings, f.token, "", "", 200), &state))
	if state.Revision != 0 || !state.AcceptingOrders || state.AcceptanceMode != "manual" {
		t.Fatal("unsafe default settings")
	}
	f.request(t, "GET", settings, "", "", "", 401)
	_, limited, err := persistence.ProvisionMerchantGrant(context.Background(), f.admin, store.MerchantID, []string{"offering:write"}, time.Now().Add(time.Hour))
	must(t, err)
	f.request(t, "PUT", settings, limited, "limited-setting-key", `{"expected_revision":0,"accepting_orders":false,"acceptance_mode":"manual"}`, 403)
	f.request(t, "PUT", settings, f.token, "invalid-setting-key", `{"expected_revision":0,"accepting_orders":false,"acceptance_mode":"other"}`, 400)
	pause := `{"expected_revision":0,"accepting_orders":false,"acceptance_mode":"automatic"}`
	first := f.request(t, "PUT", settings, f.token, "pause-ordering-key", pause, 201)
	f.request(t, "GET", public+"/offering", "", "", "", 200)
	f.request(t, "GET", public+"/food/ordering", "", "", "", 200)
	for _, path := range []string{public, guest} {
		credential := ""
		if path == guest {
			credential = token
		}
		body := f.request(t, "POST", path+"/food/quotes", credential, "", selection(1), 409)
		if !bytes.Contains(body, []byte(`"code":"ordering_paused"`)) {
			t.Fatalf("wrong pause error: %s", body)
		}
	}
	placement := fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, quote.ID)
	f.request(t, "POST", guest+"/food/orders", token, "place-paused-order", placement, 409)
	conflict := f.request(t, "PUT", settings, f.token, "stale-ordering-key", pause, 409)
	if !bytes.Contains(conflict, []byte(`"current_revision":1`)) {
		t.Fatal("missing settings conflict revision")
	}
	resume := `{"expected_revision":1,"accepting_orders":true,"acceptance_mode":"automatic"}`
	f.request(t, "PUT", settings, f.token, "resume-ordering-key", resume, 201)
	if !bytes.Equal(first, f.request(t, "PUT", settings, f.token, "pause-ordering-key", pause, 201)) {
		t.Fatal("settings replay changed")
	}
	// Replaying a historical pause must not pause again.
	must(t, json.Unmarshal(f.request(t, "GET", settings, f.token, "", "", 200), &state))
	if !state.AcceptingOrders || state.Revision != 2 {
		t.Fatal("historical replay changed live settings")
	}
	placed := f.request(t, "POST", guest+"/food/orders", token, "place-resumed-order", placement, 201)
	var order foodstore.FoodOrder
	must(t, json.Unmarshal(placed, &order))
	if order.Acceptance != "accepted" || order.Revision != 2 || order.Payment != "not_started" || order.FulfilmentProgress != "not_started" {
		t.Fatalf("wrong automatic outcome: %+v", order)
	}
	var actor string
	must(t, f.admin.QueryRow(`SELECT actor_id::text FROM commerce.food_order_events WHERE order_id=$1 AND event='accepted'`, order.ID).Scan(&actor))
	if actor != f.principal.ID.String() {
		t.Fatal("automatic acceptance attributed to customer")
	}
	// Change policy after placement: no retroactive change and no duplicate retry.
	f.request(t, "PUT", settings, f.token, "pause-again-ordering", `{"expected_revision":2,"accepting_orders":false,"acceptance_mode":"manual"}`, 201)
	if !bytes.Equal(placed, f.request(t, "POST", guest+"/food/orders", token, "place-resumed-order", placement, 201)) {
		t.Fatal("placement replay blocked by pause")
	}
	must(t, json.Unmarshal(f.request(t, "GET", guest+"/food/orders/"+order.ID.String(), token, "", "", 200), &order))
	if order.Acceptance != "accepted" || len(order.History) != 2 {
		t.Fatal("policy changed existing order or duplicated events")
	}
	f.request(t, "PUT", private+"/publication", f.token, "withdraw-paused-menu", `{"expected_generation":1}`, 201)
	f.request(t, "GET", public+"/food/ordering", "", "", "", 404)
	f.request(t, "GET", guest+"/food/orders/"+order.ID.String(), token, "", "", 200)
	var count int
	must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.food_ordering_audit`).Scan(&count))
	if count != 3 {
		t.Fatalf("duplicate or missing settings audit: %d", count)
	}
}
