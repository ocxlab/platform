package devfixtures_test

import (
	"encoding/json"
	"ocp.local/platform/internal/devfixtures"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/offering/food"
	"testing"
)

func TestDemoMealsAreValidAndRepeatable(t *testing.T) {
	m, _, err := devfixtures.Restaurant()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(m)
	var doc map[string]any
	_ = json.Unmarshal(b, &doc)
	if !devfixtures.AddDemoMeals(doc) {
		t.Fatal("not enriched")
	}
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	b, _ = json.Marshal(doc)
	if _, err = food.Validator(spec.Components.Schemas["FoodMenu"].Value)(b); err != nil {
		t.Fatal(err)
	}
	if len(doc["meal_deals"].([]any)) != 2 {
		t.Fatal("missing example deals")
	}
	if devfixtures.AddDemoMeals(doc) {
		t.Fatal("repeat changed merchant data")
	}
}
