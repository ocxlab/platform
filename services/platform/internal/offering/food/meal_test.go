package food_test

import (
	"encoding/json"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/offering"
	"ocp.local/platform/internal/offering/food"
	"strings"
	"testing"
	"time"
)

const mealMenu = `{"currency":"EUR","availability_mode":"manual","items":[{"id":"pizza","name":"Pizza","available":true,"variants":[{"id":"small","name":"Small","price_minor":800},{"id":"large","name":"Large","price_minor":1000}],"option_groups":[{"id":"extras","name":"Extras","min_selected":0,"max_selected":1,"options":[{"id":"cheese","name":"Cheese","price_minor":100,"variant_prices":{"large":150,"small":0}}]}]},{"id":"salad","name":"Salad","translations":{"ar":"سلطة"},"available":true,"variants":[{"id":"regular","name":"Regular","price_minor":650}],"option_groups":[]}],"meal_deals":[{"id":"healthy","name":"Healthy Pizza","translations":{"ar":"وجبة بيتزا"},"main_item_ids":["pizza"],"surcharge_minor":200,"addon_choices":[{"item_id":"salad","variant_id":"regular"}]}]}`

func TestMealPricesSizesAndPhysicalQuantities(t *testing.T) {
	for _, tc := range []struct {
		size string
		want int64
	}{{"large", 2700}, {"small", 2000}} {
		line := food.SelectionLine{ItemID: "pizza", VariantID: tc.size, Quantity: 2, Options: []food.SelectedOption{{GroupID: "extras", OptionID: "cheese"}}, MealDeal: &food.SelectedMealDeal{DealID: "healthy", AddonItemID: "salad", AddonVariantID: "regular"}}
		now := time.Now()
		q, total, expiry, err := food.PriceModel(food.Model, []byte(mealMenu), []food.SelectionLine{line}, "ar", now)
		if err != nil || total != tc.want || !expiry.Equal(now.Add(10*time.Minute)) {
			t.Fatalf("%s %d %v", tc.size, total, err)
		}
		if q[0].MealDeal.AddonQuantity != 2 || q[0].MealDeal.AddonItemName != "سلطة" || q[0].MealDeal.Name != "وجبة بيتزا" {
			t.Fatalf("lost physical/localized terms %+v", q[0].MealDeal)
		}
	}
}

func TestMealValidationAndTampering(t *testing.T) {
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	validate := food.Validator(spec.Components.Schemas["FoodMenu"].Value)
	if _, err = validate([]byte(mealMenu)); err != nil {
		t.Fatal(err)
	}
	for _, replacement := range [][2]string{
		{`"main_item_ids":["pizza"]`, `"main_item_ids":["missing"]`},
		{`"variant_prices":{"large":150,"small":0}`, `"variant_prices":{"unknown":150}`},
		{`"addon_choices":[{"item_id":"salad","variant_id":"regular"}]`, `"addon_choices":[{"item_id":"salad","variant_id":"bad"}]`},
		{`"surcharge_minor":200`, `"surcharge_minor":-1`},
		{`"main_item_ids":["pizza"]`, `"main_item_ids":["pizza","pizza"]`},
		{`"currency":"EUR"`, `"time_zone":"Europe/Berlin","currency":"EUR"`},
	} {
		if _, err = validate([]byte(strings.Replace(mealMenu, replacement[0], replacement[1], 1))); err == nil {
			t.Fatalf("accepted %s", replacement[1])
		}
	}
	line := food.SelectionLine{ItemID: "pizza", VariantID: "large", Quantity: 1, Options: []food.SelectedOption{}, MealDeal: &food.SelectedMealDeal{DealID: "healthy", AddonItemID: "salad", AddonVariantID: "regular"}}
	for _, bad := range []food.SelectedMealDeal{{DealID: "other", AddonItemID: "salad", AddonVariantID: "regular"}, {DealID: "healthy", AddonItemID: "pizza", AddonVariantID: "large"}, {DealID: "healthy", AddonItemID: "salad", AddonVariantID: "bad"}} {
		copy := line
		copy.MealDeal = &bad
		if _, _, err = food.Price([]byte(mealMenu), []food.SelectionLine{copy}); err == nil {
			t.Fatal("accepted tampering")
		}
	}
	var doc map[string]any
	_ = json.Unmarshal([]byte(mealMenu), &doc)
	addon := doc["items"].([]any)[1].(map[string]any)
	addon["available"] = false
	b, _ := json.Marshal(doc)
	if _, _, err = food.Price(b, []food.SelectionLine{line}); err == nil {
		t.Fatal("sold unavailable addon")
	}
	addon["available"] = true
	addon["option_groups"] = []any{map[string]any{"id": "dressing", "name": "Dressing", "min_selected": 1, "max_selected": 1, "options": []any{map[string]any{"id": "oil", "name": "Oil", "price_minor": 0}}}}
	b, _ = json.Marshal(doc)
	if _, err = validate(b); err == nil {
		t.Fatal("included required modifier accepted")
	}
	if _, _, err = food.Price(b, []food.SelectionLine{line}); err == nil {
		t.Fatal("pricer accepted required addon modifier")
	}
	if _, _, _, err = food.PriceModel(offering.Model{ID: food.Model.ID, Version: "2.0.0"}, []byte(mealMenu), []food.SelectionLine{line}, "", time.Now()); err == nil {
		t.Fatal("retired model admitted")
	}
}

func TestMealSurchargeBoundariesAndExtraFallback(t *testing.T) {
	line := food.SelectionLine{ItemID: "pizza", VariantID: "large", Quantity: 1, Options: []food.SelectedOption{{GroupID: "extras", OptionID: "cheese"}}, MealDeal: &food.SelectedMealDeal{DealID: "healthy", AddonItemID: "salad", AddonVariantID: "regular"}}
	for _, surcharge := range []string{"0", "1000000000", "-1", "1000000001"} {
		raw := strings.Replace(mealMenu, `"surcharge_minor":200`, `"surcharge_minor":`+surcharge, 1)
		_, total, err := food.Price([]byte(raw), []food.SelectionLine{line})
		switch surcharge {
		case "0":
			if err != nil || total != 1150 {
				t.Fatalf("zero surcharge %d %v", total, err)
			}
		case "1000000000":
			if err != nil || total != 1000001150 {
				t.Fatalf("max surcharge %d %v", total, err)
			}
		default:
			if err == nil {
				t.Fatal("out of range surcharge accepted")
			}
		}
	}
	// A size missing from the override map must use the original extra price.
	raw := strings.Replace(mealMenu, `"variant_prices":{"large":150,"small":0}`, `"variant_prices":{"small":0}`, 1)
	quoted, total, err := food.Price([]byte(raw), []food.SelectionLine{line})
	if err != nil || total != 1300 || quoted[0].SelectedOptions[0].Price == nil || *quoted[0].SelectedOptions[0].Price != 100 {
		t.Fatalf("fallback %d %v %+v", total, err, quoted)
	}
}

func TestIncludedItemOptionalDefaultsAreNotPurchased(t *testing.T) {
	var doc map[string]any
	_ = json.Unmarshal([]byte(mealMenu), &doc)
	addon := doc["items"].([]any)[1].(map[string]any)
	addon["option_groups"] = []any{map[string]any{"id": "dressing", "name": "Dressing", "min_selected": 0, "max_selected": 1, "default_option_ids": []string{"oil"}, "options": []any{map[string]any{"id": "oil", "name": "Oil", "price_minor": 100}}}}
	raw, _ := json.Marshal(doc)
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = food.Validator(spec.Components.Schemas["FoodMenu"].Value)(raw); err != nil {
		t.Fatal(err)
	}
	line := food.SelectionLine{ItemID: "pizza", VariantID: "large", Quantity: 1, Options: []food.SelectedOption{}, MealDeal: &food.SelectedMealDeal{DealID: "healthy", AddonItemID: "salad", AddonVariantID: "regular"}}
	quoted, total, err := food.Price(raw, []food.SelectionLine{line})
	if err != nil || total != 1200 || len(quoted[0].SelectedOptions) != 0 || quoted[0].MealDeal.AddonQuantity != 1 {
		t.Fatalf("unexpected optional addon default %d %v %+v", total, err, quoted)
	}
}
