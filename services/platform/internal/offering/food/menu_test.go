package food_test

import (
	"encoding/json"
	"testing"

	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/offering/food"
)

func TestMenuSemanticValidation(t *testing.T) {
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	validate := food.Validator(spec.Components.Schemas["FoodMenu"].Value)
	raw := []byte(`{"currency":"EUR","availability_mode":"manual","items":[{"id":"meal","name":"Meal","available":true,"variants":[{"id":"regular","name":"Regular","price_minor":700}],"option_groups":[{"id":"extra","name":"Extra","min_selected":1,"max_selected":1,"options":[{"id":"cheese","name":"Cheese","price_minor":50}]}]}]}`)
	if _, err := validate(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := validate([]byte(`{`)); err == nil {
		t.Fatal("invalid JSON accepted")
	}
	if _, err := validate([]byte(`{"currency":"USD","items":[]}`)); err == nil {
		t.Fatal("invalid schema accepted")
	}
	for _, name := range []string{"duplicate-item", "blank-item", "duplicate-variant", "blank-variant", "duplicate-group", "blank-group", "min-over-max", "max-over-count", "duplicate-option", "blank-option"} {
		t.Run(name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			items := doc["items"].([]any)
			item := items[0].(map[string]any)
			variants := item["variants"].([]any)
			groups := item["option_groups"].([]any)
			group := groups[0].(map[string]any)
			options := group["options"].([]any)
			switch name {
			case "duplicate-item":
				doc["items"] = append(items, items[0])
			case "blank-item":
				item["name"] = "  "
			case "duplicate-variant":
				item["variants"] = append(variants, variants[0])
			case "blank-variant":
				variants[0].(map[string]any)["name"] = "  "
			case "duplicate-group":
				item["option_groups"] = append(groups, groups[0])
			case "blank-group":
				group["name"] = "  "
			case "min-over-max":
				group["min_selected"] = float64(2)
				group["max_selected"] = float64(1)
			case "max-over-count":
				group["max_selected"] = float64(len(options) + 1)
			case "duplicate-option":
				group["options"] = append(options, options[0])
			case "blank-option":
				options[0].(map[string]any)["name"] = "  "
			}
			mutated, _ := json.Marshal(doc)
			if _, err := validate(mutated); err == nil {
				t.Fatal("invalid menu accepted")
			}
		})
	}
}
