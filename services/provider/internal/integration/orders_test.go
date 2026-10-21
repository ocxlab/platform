package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

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
	if order.Terms.Total != 1400 || order.Acceptance != "pending" || order.Payment != "not_started" || order.FulfilmentProgress != "not_started" {
		t.Fatalf("incorrect placement: %+v", order)
	}
	f.request(t, "GET", path+"/"+order.ID.String(), other.Token, "", "", 404)
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
	_, err = f.admin.Exec(`UPDATE commerce.guest_grants SET revoked=true WHERE merchant_id=$1`, store.MerchantID)
	must(t, err)
	f.request(t, "GET", path+"/"+order.ID.String(), session.Token, "", "", 401)
	f.request(t, "POST", path, session.Token, "place-food-order-001", body, 401)
}
