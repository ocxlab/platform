package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"ocp.local/platform/internal/offering/food"
	"ocp.local/platform/internal/persistence"
)

func TestFoodV2HTTPJourney(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	private := f.merchantPath + "/" + store.ID.String()
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	guest := strings.Replace(private, "/api/v1/", "/api/v1/guest/", 1)
	doc := menu(t)
	doc["time_zone"] = "Europe/Berlin"
	item := doc["items"].([]any)[0].(map[string]any)
	item["option_groups"].([]any)[0].(map[string]any)["options"].([]any)[0].(map[string]any)["variant_prices"] = map[string]int{"regular": 100}
	doc["offers"] = []any{map[string]any{"id": "demo-offer", "item_id": "doener", "variant_id": "regular", "price_minor": 500, "priority": 1, "windows": []any{map[string]any{"weekdays": []int{1, 2, 3, 4, 5, 6, 7}, "start": "00:00", "end": "24:00"}}}}
	write := func(base int) string {
		b, _ := json.Marshal(map[string]any{"base_revision": base, "model": food.ModelV2, "document": doc})
		return string(b)
	}
	f.request(t, "PUT", private+"/offering", f.token, "v2-write-menu-0001", write(1), 201)
	f.request(t, "PUT", private+"/publication", f.token, "v2-publish-menu-0001", `{"expected_generation":0,"revision":2}`, 201)
	var session persistence.GuestSession
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &session))
	var q persistence.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", session.Token, "", selection(1), 201), &q))
	if q.Model != food.ModelV2 || q.Total != 1200 || q.ExpiresAt.After(q.CreatedAt.Add(10*time.Minute)) {
		t.Fatalf("wrong V2 quote %+v", q)
	}
	var lines []food.QuotedLine
	must(t, json.Unmarshal(q.Lines, &lines))
	if lines[0].PriceExplanation == nil || lines[0].PriceExplanation.OfferID != "demo-offer" {
		t.Fatal("lost pricing facts")
	}
	// A later price-only revision must not rewrite the customer's agreed quote.
	doc["offers"].([]any)[0].(map[string]any)["price_minor"] = 800
	f.request(t, "PUT", private+"/offering", f.token, "v2-write-menu-0002", write(2), 201)
	f.request(t, "PUT", private+"/publication", f.token, "v2-publish-menu-0002", `{"expected_generation":1,"revision":3}`, 201)
	body := fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, q.ID)
	first := f.request(t, "POST", guest+"/food/orders", session.Token, "v2-place-order-0001", body, 201)
	var order persistence.FoodOrder
	must(t, json.Unmarshal(first, &order))
	if order.Terms.Total != 1200 {
		t.Fatal("repriced issued quote")
	}
	if !bytes.Equal(first, f.request(t, "POST", guest+"/food/orders", session.Token, "v2-place-order-0001", body, 201)) {
		t.Fatal("replay changed")
	}
	// V1 revisions stay independently readable and publishable.
	var pendingV2 persistence.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", session.Token, "", selection(2), 201), &pendingV2))
	f.request(t, "PUT", private+"/publication", f.token, "v2-back-to-v1-0001", `{"expected_generation":2,"revision":1}`, 201)
	f.request(t, "POST", guest+"/food/orders", session.Token, "v2-cross-model-0001", fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, pendingV2.ID), 422)
	var v1 persistence.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", public+"/food/quotes", "", "", selection(3), 201), &v1))
	if v1.Model != food.Model || v1.Total != 1400 {
		t.Fatal("V1 no longer works")
	}
	var pendingV1 persistence.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", session.Token, "", selection(3), 201), &pendingV1))
	f.request(t, "PUT", private+"/publication", f.token, "v2-return-new-model-0001", `{"expected_generation":3,"revision":3}`, 201)
	f.request(t, "POST", guest+"/food/orders", session.Token, "v1-cross-model-0001", fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, pendingV1.ID), 422)
}
