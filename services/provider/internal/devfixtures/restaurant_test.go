package devfixtures_test

import (
	"encoding/json"
	"ocp.local/platform/internal/devfixtures"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/offering/food"
	"testing"
)

func TestRestaurantUsesInstalledMenuModel(t *testing.T) {
	menu, products, err := devfixtures.Restaurant()
	if err != nil {
		t.Fatal(err)
	}
	if len(menu.Items) != 48 || len(products) != 48 {
		t.Fatal("expected 48 distinct demo products")
	}
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(menu)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = food.Validator(spec.Components.Schemas["FoodMenu"].Value)(raw); err != nil {
		t.Fatal(err)
	}
	sections := map[string]bool{}
	for _, p := range products {
		sections[p.Section] = true
	}
	if len(sections) != 7 {
		t.Fatalf("expected seven intended sections: %v", sections)
	}
	if menu.Items[46].Available {
		t.Fatal("baklava must exercise sold-out behavior")
	}
}

// A small, explicit mapping of public menu facts, not a whole-menu import or
// proof of the restaurant's actual modifier rules. See the benchmark note.
func TestPublishedMenuSimplePriceCases(t *testing.T) {
	menu := devfixtures.Menu{Currency: "EUR", Availability: "manual", Items: []devfixtures.Item{
		{ID: "pommes", Name: "Pommes", Available: true, Variants: []devfixtures.Variant{{ID: "regular", Name: "Portion", Price: 400}}, Groups: []devfixtures.OptionGroup{}},
		{ID: "bulgur", Name: "Bulgur", Available: true, Variants: []devfixtures.Variant{{ID: "regular", Name: "Portion", Price: 400}}, Groups: []devfixtures.OptionGroup{}},
		{ID: "pizza-margherita", Name: "Pizza Margherita", Available: true, Variants: []devfixtures.Variant{{ID: "30cm", Name: "30 cm", Price: 900}}, Groups: []devfixtures.OptionGroup{}},
	}}
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(menu)
	if _, err = food.Validator(spec.Components.Schemas["FoodMenu"].Value)(raw); err != nil {
		t.Fatal(err)
	}
	// Category metadata cannot yet round-trip through the current closed schema.
	var document map[string]any
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	document["categories"] = []any{map[string]any{"id": "sides", "name": "Beilagen"}}
	raw, _ = json.Marshal(document)
	if _, err = food.Validator(spec.Components.Schemas["FoodMenu"].Value)(raw); err == nil {
		t.Fatal("do not hide the missing category model")
	}
}
