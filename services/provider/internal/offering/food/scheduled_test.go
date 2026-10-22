package food_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/offering/food"
)

const scheduledMenu = `{"currency":"EUR","availability_mode":"manual","time_zone":"Europe/Berlin","items":[{"id":"pizza","name":"Pizza","translations":{"en":"Pizza EN"},"available":true,"variants":[{"id":"small","name":"Small","price_minor":800},{"id":"large","name":"Large","price_minor":1200}],"option_groups":[{"id":"extras","name":"Extras","min_selected":0,"max_selected":1,"options":[{"id":"cheese","name":"Cheese","price_minor":100,"variant_prices":{"large":200}}]}]}],"offers":[{"id":"lunch","item_id":"pizza","variant_id":"large","price_minor":1000,"priority":10,"windows":[{"weekdays":[1,2,3,4,5],"start":"12:00","end":"14:00"}]}]}`

func instant(s string) time.Time {
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return v
}
func TestScheduledPricing(t *testing.T) {
	lines := []food.SelectionLine{{ItemID: "pizza", VariantID: "large", Quantity: 2, Options: []food.SelectedOption{{GroupID: "extras", OptionID: "cheese"}}}}
	for _, test := range []struct {
		name, at string
		total    int64
		expiry   string
		offer    string
	}{
		{"before", "2026-10-22T09:55:30Z", 2800, "2026-10-22T10:00:00Z", ""},
		{"start", "2026-10-22T10:00:00Z", 2400, "2026-10-22T10:10:00Z", "lunch"},
		{"end-soon", "2026-10-22T11:59:30Z", 2400, "2026-10-22T12:00:00Z", "lunch"},
		{"end", "2026-10-22T12:00:00Z", 2800, "2026-10-22T12:10:00Z", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			q, total, expiry, err := food.PriceModel(food.ModelV2, []byte(scheduledMenu), lines, "en", instant(test.at))
			if err != nil || total != test.total || !expiry.Equal(instant(test.expiry)) {
				t.Fatalf("%+v %d %v %v", q, total, expiry, err)
			}
			e := q[0].PriceExplanation
			if e == nil || e.OfferID != test.offer || e.Base != 1200 || e.Extras != 200 || q[0].ItemName != "Pizza EN" || q[0].SelectedOptions[0].Price == nil || *q[0].SelectedOptions[0].Price != 200 {
				t.Fatalf("explanation %+v / %+v", e, q)
			}
		})
	}
	// Distinct sizes and repeated lines must not share one materialized extra price.
	lines = append(lines, food.SelectionLine{ItemID: "pizza", VariantID: "small", Quantity: 1, Options: lines[0].Options}, lines[0])
	_, total, _, err := food.PriceModel(food.ModelV2, []byte(scheduledMenu), lines, "", instant("2026-10-22T10:00:00Z"))
	if err != nil || total != 5700 {
		t.Fatalf("mixed sizes %d %v", total, err)
	}
}

func TestOfferPrecedenceAndCombinedAvailability(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(scheduledMenu), &doc); err != nil {
		t.Fatal(err)
	}
	windows := []any{map[string]any{"weekdays": []int{4}, "start": "12:00", "end": "14:00"}}
	lines := []food.SelectionLine{{ItemID: "pizza", VariantID: "large", Quantity: 1}}
	offer := func(id string, priority, price int) any {
		return map[string]any{"id": id, "item_id": "pizza", "variant_id": "large", "priority": priority, "price_minor": price, "windows": windows}
	}
	for _, test := range []struct {
		offers []any
		want   int64
		id     string
	}{
		{[]any{offer("a", 1, 100), offer("b", 2, 200)}, 200, "b"},
		{[]any{offer("b", 2, 200), offer("a", 2, 100)}, 100, "a"},
		{[]any{offer("a", 2, 100), offer("b", 2, 200)}, 100, "a"},
	} {
		doc["offers"] = test.offers
		raw, _ := json.Marshal(doc)
		q, total, _, err := food.PriceModel(food.ModelV2, raw, lines, "", instant("2026-10-22T10:30:00Z"))
		if err != nil || total != test.want || q[0].PriceExplanation.OfferID != test.id {
			t.Fatalf("precedence: %v %d %v", q, total, err)
		}
	}
	item := doc["items"].([]any)[0].(map[string]any)
	variant := item["variants"].([]any)[1].(map[string]any)
	item["availability_windows"] = windows
	variant["availability_windows"] = []any{map[string]any{"weekdays": []int{4}, "start": "13:00", "end": "14:00"}}
	for _, test := range []struct {
		at            string
		manual, valid bool
	}{{"2026-10-22T10:30:00Z", true, false}, {"2026-10-22T11:30:00Z", true, true}, {"2026-10-22T11:30:00Z", false, false}, {"2026-10-22T12:30:00Z", true, false}} {
		item["available"] = test.manual
		raw, _ := json.Marshal(doc)
		_, _, _, err := food.PriceModel(food.ModelV2, raw, lines, "", instant(test.at))
		if (err == nil) != test.valid {
			t.Fatalf("availability %+v: %v", test, err)
		}
	}
}

func TestV2ValidationAndV1Isolation(t *testing.T) {
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	validate := food.ValidatorV2(spec.Components.Schemas["FoodMenuV2"].Value, spec.Components.Schemas["FoodMenu"].Value)
	if _, err = validate([]byte(scheduledMenu)); err != nil {
		t.Fatal(err)
	}
	if _, err = food.Validator(spec.Components.Schemas["FoodMenu"].Value)([]byte(scheduledMenu)); err == nil {
		t.Fatal("V1 accepted V2")
	}
	for _, pair := range [][2]string{
		{`"Europe/Berlin"`, `"Not/AZone"`}, {`"Europe/Berlin"`, `"Local"`}, {`"large":200`, `"missing":200`}, {`"variant_id":"large"`, `"variant_id":"missing"`},
		{`"priority":10`, `"priority":1001`}, {`"start":"12:00"`, `"start":"14:00"`}, {`"start":"12:00"`, `"start":"24:00"`},
		{`"weekdays":[1,2,3,4,5]`, `"weekdays":[0]`}, {`"price_minor":1000`, `"price_minor":-1`},
		{`"end":"14:00"`, `"end":"14:00","valid_from":"2026-10-23T00:00:00Z","valid_until":"2026-10-22T00:00:00Z"`},
		{`"end":"14:00"`, `"end":"14:00","valid_from":"2026-10-22T00:00:00+00:00"`},
		{`"name":"Pizza"`, `"name":" "`},
	} {
		if _, err = validate([]byte(strings.Replace(scheduledMenu, pair[0], pair[1], 1))); err == nil {
			t.Errorf("accepted %s", pair[1])
		}
	}
	projected, err := food.Project([]byte(scheduledMenu))
	if err != nil || !strings.Contains(string(projected), `"variant_prices"`) || !strings.Contains(string(projected), `"time_zone"`) {
		t.Fatalf("lost V2 projection %s %v", projected, err)
	}
}

func TestScheduleAvailabilityAndDST(t *testing.T) {
	var doc map[string]any
	_ = json.Unmarshal([]byte(scheduledMenu), &doc)
	item := doc["items"].([]any)[0].(map[string]any)
	item["availability_windows"] = []any{map[string]any{"weekdays": []int{7}, "start": "02:00", "end": "03:00"}}
	doc["offers"] = []any{}
	raw, _ := json.Marshal(doc)
	lines := []food.SelectionLine{{ItemID: "pizza", VariantID: "large", Quantity: 1}}
	for _, at := range []string{"2026-10-25T00:30:00Z", "2026-10-25T01:30:00Z"} {
		if _, _, _, err := food.PriceModel(food.ModelV2, raw, lines, "", instant(at)); err != nil {
			t.Fatalf("repeated wall time unavailable %s", at)
		}
	}
	for _, at := range []string{"2026-10-25T02:00:00Z", "2026-03-29T01:00:00Z"} {
		if _, _, _, err := food.PriceModel(food.ModelV2, raw, lines, "", instant(at)); err == nil {
			t.Fatalf("outside window available %s", at)
		}
	}
	// An explicit UTC campaign cutoff must shorten a quote even between minutes.
	item["availability_windows"] = []any{map[string]any{"weekdays": []int{4}, "start": "00:00", "end": "24:00", "valid_until": "2026-10-22T10:00:12Z"}}
	raw, _ = json.Marshal(doc)
	_, _, expiry, err := food.PriceModel(food.ModelV2, raw, lines, "", instant("2026-10-22T10:00:00Z"))
	if err != nil || !expiry.Equal(instant("2026-10-22T10:00:12Z")) {
		t.Fatalf("UTC edge %v %v", expiry, err)
	}
}
