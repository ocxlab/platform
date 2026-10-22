package food_test

import (
	"encoding/json"
	"ocp.local/platform/internal/devfixtures"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/offering/food"
	"testing"
)

func TestCategoriesValidateAndProjectWithoutChangingPrices(t *testing.T) {
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	validate := food.Validator(spec.Components.Schemas["FoodMenu"].Value)
	fixture, _, err := devfixtures.Restaurant()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(fixture)
	canonical, err := validate(raw)
	if err != nil {
		t.Fatal(err)
	}
	projected, err := food.Project(canonical)
	if err != nil {
		t.Fatal(err)
	}
	var published devfixtures.Menu
	if err := json.Unmarshal(projected, &published); err != nil {
		t.Fatal(err)
	}
	if len(published.Categories) != 7 || published.Items[0].CategoryID != fixture.Items[0].CategoryID {
		t.Fatal("lost public categories")
	}
	for name, change := range map[string]func(*devfixtures.Menu){
		"duplicate": func(m *devfixtures.Menu) { m.Categories = append(m.Categories, m.Categories[0]) },
		"blank":     func(m *devfixtures.Menu) { m.Categories[0].Name = "   " },
		"dangling":  func(m *devfixtures.Menu) { m.Items[0].CategoryID = "missing" },
	} {
		t.Run(name, func(t *testing.T) {
			var m devfixtures.Menu
			json.Unmarshal(raw, &m)
			change(&m)
			invalid, _ := json.Marshal(m)
			if _, err := validate(invalid); err == nil {
				t.Fatal("invalid category accepted")
			}
		})
	}
	line := []food.SelectionLine{{ItemID: "pizza-margherita", VariantID: "small", Quantity: 1}}
	_, before, err := food.Price(raw, line)
	if err != nil {
		t.Fatal(err)
	}
	fixture.Categories = nil
	for i := range fixture.Items {
		fixture.Items[i].CategoryID = ""
	}
	legacy, _ := json.Marshal(fixture)
	if _, err := validate(legacy); err != nil {
		t.Fatal(err)
	}
	_, after, err := food.Price(legacy, line)
	if err != nil || before != after {
		t.Fatal("category changed price", err)
	}
}
