package food

import (
	"encoding/json"
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
