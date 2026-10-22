package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"ocp.local/platform/internal/offering/food"
	"ocp.local/platform/internal/persistence"
	"strings"
	"testing"
)

func TestFoodMealHTTPJourney(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	private := f.merchantPath + "/" + store.ID.String()
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	guest := strings.Replace(private, "/api/v1/", "/api/v1/guest/", 1)
	doc := menu(t)
	item := doc["items"].([]any)[0].(map[string]any)
	item["option_groups"].([]any)[0].(map[string]any)["options"].([]any)[0].(map[string]any)["variant_prices"] = map[string]int{"regular": 100}
	doc["items"] = append(doc["items"].([]any), map[string]any{"id": "drink", "name": "Ayran", "available": true, "variants": []any{map[string]any{"id": "bottle", "name": "250 ml", "price_minor": 250}}, "option_groups": []any{}})
	doc["meal_deals"] = []any{map[string]any{"id": "doner-meal", "name": "Döner menu", "main_item_ids": []string{"doener"}, "surcharge_minor": 100, "addon_choices": []any{map[string]any{"item_id": "drink", "variant_id": "bottle"}}}}
	write := func(base int, version string) string {
		b, _ := json.Marshal(map[string]any{"base_revision": base, "model": map[string]string{"id": "ocp.food.menu", "version": version}, "document": doc})
		return string(b)
	}
	f.request(t, "PUT", private+"/offering", f.token, "meal-no-v2-version-0001", write(1, "2.0.0"), 422)
	f.request(t, "PUT", private+"/offering", f.token, "meal-write-menu-0001", write(1, "1.0.0"), 201)
	f.request(t, "PUT", private+"/publication", f.token, "meal-publish-0001", `{"expected_generation":0,"revision":2}`, 201)
	var session persistence.GuestSession
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &session))
	selected := map[string]any{}
	must(t, json.Unmarshal([]byte(selection(1)), &selected))
	line := selected["lines"].([]any)[0].(map[string]any)
	line["meal_deal"] = map[string]string{"deal_id": "doner-meal", "addon_item_id": "drink", "addon_variant_id": "bottle"}
	body, _ := json.Marshal(selected)
	var q persistence.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", session.Token, "", string(body), 201), &q))
	var quoted []food.QuotedLine
	must(t, json.Unmarshal(q.Lines, &quoted))
	if q.Model != food.Model || q.Total != 1800 || quoted[0].MealDeal == nil || quoted[0].MealDeal.AddonQuantity != 2 {
		t.Fatalf("meal quote %+v %s", q, q.Lines)
	}
	// Price-only publication preserves the issued price and physical included item.
	doc["meal_deals"].([]any)[0].(map[string]any)["surcharge_minor"] = 300
	f.request(t, "PUT", private+"/offering", f.token, "meal-write-menu-0002", write(2, "1.0.0"), 201)
	f.request(t, "PUT", private+"/publication", f.token, "meal-publish-0002", `{"expected_generation":1,"revision":3}`, 201)
	orderBody := fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, q.ID)
	first := f.request(t, "POST", guest+"/food/orders", session.Token, "meal-place-order-0001", orderBody, 201)
	if !bytes.Equal(first, f.request(t, "POST", guest+"/food/orders", session.Token, "meal-place-order-0001", orderBody, 201)) {
		t.Fatal("changed replay")
	}
	var order persistence.FoodOrder
	must(t, json.Unmarshal(first, &order))
	if order.Terms.Total != 1800 {
		t.Fatal("repriced terms")
	}
	// Availability of the physical included product is rechecked at placement.
	selected["publication_generation"] = 2
	body, _ = json.Marshal(selected)
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", session.Token, "", string(body), 201), &q))
	doc["items"].([]any)[1].(map[string]any)["available"] = false
	f.request(t, "PUT", private+"/offering", f.token, "meal-write-menu-0003", write(3, "1.0.0"), 201)
	f.request(t, "PUT", private+"/publication", f.token, "meal-publish-0003", `{"expected_generation":2,"revision":4}`, 201)
	f.request(t, "POST", guest+"/food/orders", session.Token, "meal-unavailable-0001", fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, q.ID), 422)
	// Removing membership also invalidates an issued quote, even when all items
	// remain available. This is eligibility enforcement, not repricing.
	doc["items"].([]any)[1].(map[string]any)["available"] = true
	f.request(t, "PUT", private+"/offering", f.token, "meal-write-menu-0004", write(4, "1.0.0"), 201)
	f.request(t, "PUT", private+"/publication", f.token, "meal-publish-0004", `{"expected_generation":3,"revision":5}`, 201)
	selected["publication_generation"] = 4
	body, _ = json.Marshal(selected)
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", session.Token, "", string(body), 201), &q))
	doc["meal_deals"] = []any{}
	f.request(t, "PUT", private+"/offering", f.token, "meal-write-menu-0005", write(5, "1.0.0"), 201)
	f.request(t, "PUT", private+"/publication", f.token, "meal-publish-0005", `{"expected_generation":4,"revision":6}`, 201)
	f.request(t, "POST", guest+"/food/orders", session.Token, "meal-membership-0001", fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, q.ID), 422)
}
