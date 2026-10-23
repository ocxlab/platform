package food

import (
	"encoding/json"
	"strings"
	"testing"
)

const publicMenu = `{"currency":"EUR","availability_mode":"manual","items":[{"id":"meal","name":"Meal","available":true,"variants":[{"id":"regular","name":"Regular","price_minor":700}],"option_groups":[{"id":"extra","name":"Extra","min_selected":1,"max_selected":1,"options":[{"id":"cheese","name":"Cheese","price_minor":50}]}]}]}`

func TestSelectionPrice(t *testing.T) {
	good := SelectionLine{ItemID: "meal", VariantID: "regular", Quantity: 2, Options: []SelectedOption{{"extra", "cheese"}}}
	lines, total, err := Price([]byte(publicMenu), []SelectionLine{good})
	if err != nil || total != 1500 || lines[0].UnitPrice != 750 {
		t.Fatalf("wrong price: %v %d %v", lines, total, err)
	}
	for _, name := range []string{"unavailable", "wrong_item", "wrong_variant", "missing_required", "duplicate", "unknown_group", "unknown_option", "quantity", "arithmetic"} {
		t.Run(name, func(t *testing.T) {
			line := good
			line.Options = append([]SelectedOption{}, good.Options...)
			var doc map[string]any
			_ = json.Unmarshal([]byte(publicMenu), &doc)
			item := doc["items"].([]any)[0].(map[string]any)
			switch name {
			case "unavailable":
				item["available"] = false
			case "wrong_item":
				line.ItemID = "none"
			case "wrong_variant":
				line.VariantID = "none"
			case "missing_required":
				line.Options = nil
			case "duplicate":
				line.Options = append(line.Options, line.Options[0])
			case "unknown_group":
				line.Options[0].GroupID = "none"
			case "unknown_option":
				line.Options[0].OptionID = "none"
			case "quantity":
				line.Quantity = 100
			case "arithmetic":
				item["variants"].([]any)[0].(map[string]any)["price_minor"] = MaxTotal
				line.Quantity = 2
			}
			raw, _ := json.Marshal(doc)
			if _, _, err := Price(raw, []SelectionLine{line}); err == nil {
				t.Fatal("invalid selection accepted")
			}
		})
	}
}

func TestSelectionNoteAndMoneyBoundaries(t *testing.T) {
	line := SelectionLine{ItemID: "meal", VariantID: "regular", Quantity: 1, Options: []SelectedOption{{"extra", "cheese"}}}
	for _, n := range []int{499, 500, 501} {
		line.Note = strings.Repeat("ش", n)
		_, _, err := Price([]byte(publicMenu), []SelectionLine{line})
		if (err == nil) != (n <= 500) {
			t.Fatalf("note length %d: %v", n, err)
		}
	}
	line.Note = ""
	var value map[string]any
	_ = json.Unmarshal([]byte(publicMenu), &value)
	item := value["items"].([]any)[0].(map[string]any)
	item["variants"].([]any)[0].(map[string]any)["price_minor"] = MaxTotal - 50
	raw, _ := json.Marshal(value)
	if _, total, err := Price(raw, []SelectionLine{line}); err != nil || total != MaxTotal {
		t.Fatalf("exact money limit: %d %v", total, err)
	}
	if _, _, err := Price(raw, []SelectionLine{line, line}); err == nil {
		t.Fatal("sum exceeded money limit")
	}
	item["variants"].([]any)[0].(map[string]any)["price_minor"] = MaxTotal - 49
	raw, _ = json.Marshal(value)
	if _, _, err := Price(raw, []SelectionLine{line}); err == nil {
		t.Fatal("option exceeded money limit")
	}
}

func TestLocalizedPriceFallbacks(t *testing.T) {
	line := SelectionLine{ItemID: "meal", VariantID: "regular", Quantity: 1, Options: []SelectedOption{{"extra", "cheese"}}}
	raw := []byte(strings.Replace(publicMenu, `"name":"Meal"`, `"name":"Meal","translations":{"de":"Essen","ar":""}`, 1))
	for _, tc := range []struct{ locale, name string }{{"de", "Essen"}, {"ar", "Meal"}, {"en", "Meal"}, {"", "Meal"}, {"xx", "Meal"}} {
		lines, total, err := PriceLocalized(raw, []SelectionLine{line}, tc.locale)
		if err != nil || total != 750 || lines[0].ItemName != tc.name {
			t.Fatalf("locale %q: %+v %d %v", tc.locale, lines, total, err)
		}
	}
	if _, _, err := PriceLocalized([]byte(`{"items":"not-an-array"}`), []SelectionLine{line}, "de"); err == nil {
		t.Fatal("invalid menu accepted")
	}
}
func TestProjectionDoesNotCopyUnlistedFields(t *testing.T) {
	var value map[string]any
	_ = json.Unmarshal([]byte(publicMenu), &value)
	value["private_note"] = "not-public"
	raw, _ := json.Marshal(value)
	public, err := Project(raw)
	if err != nil {
		t.Fatal(err)
	}
	var projected map[string]any
	_ = json.Unmarshal(public, &projected)
	if _, ok := projected["private_note"]; ok {
		t.Fatal("private field leaked")
	}
}
