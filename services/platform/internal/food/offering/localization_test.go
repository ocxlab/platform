package food_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"ocp.local/platform/cmd/api/transport/httpapi"
	"ocp.local/platform/internal/food/offering"
)

func TestLocalizedPublicationAndQuote(t *testing.T) {
	raw := json.RawMessage(`{"currency":"EUR","availability_mode":"manual","items":[{"id":"pizza","name":"Pizza","translations":{"ar":"بيتزا","en":"Pizza"},"available":true,"variants":[{"id":"regular","name":"Normal","translations":{"en":"Regular"},"price_minor":800}],"option_groups":[]}]}`)
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	validate := food.Validator(spec.Components.Schemas["FoodMenu"].Value)
	if _, err := validate(raw); err != nil {
		t.Fatal(err)
	}
	projected, err := food.Project(raw)
	if err != nil || !bytes.Contains(projected, []byte(`"ar":"بيتزا"`)) {
		t.Fatalf("lost translations: %s %v", projected, err)
	}
	selection := []food.SelectionLine{{ItemID: "pizza", VariantID: "regular", Quantity: 2, Note: "Please slice", Options: []food.SelectedOption{}}}
	lines, total, err := food.PriceLocalized(raw, selection, "ar")
	if err != nil || total != 1600 || lines[0].ItemName != "بيتزا" || lines[0].VariantName != "Normal" || lines[0].Selection.Note != "Please slice" {
		t.Fatalf("bad localized quote: %+v %d %v", lines, total, err)
	}
	for _, replacement := range []string{`"translations":{"fr":"Pizza"}`, `"translations":{"en":"  "}`} {
		invalid := bytes.Replace(raw, []byte(`"translations":{"ar":"بيتزا","en":"Pizza"}`), []byte(replacement), 1)
		if _, err := validate(invalid); err == nil {
			t.Fatalf("accepted invalid translation: %s", invalid)
		}
	}
	selection[0].Note = string(bytes.Repeat([]byte("x"), 501))
	if _, _, err := food.PriceLocalized(raw, selection, "en"); err == nil {
		t.Fatal("accepted oversized note")
	}
	group := `"option_groups":[{"id":"sauce","name":"Sauce","min_selected":1,"max_selected":1,"default_option_ids":["garlic"],"options":[{"id":"garlic","name":"Garlic","price_minor":0}]}]`
	withDefault := bytes.Replace(raw, []byte(`"option_groups":[]`), []byte(group), 1)
	if _, err := validate(withDefault); err != nil {
		t.Fatalf("valid default rejected: %v", err)
	}
	projected, err = food.Project(withDefault)
	if err != nil || !bytes.Contains(projected, []byte(`"default_option_ids":["garlic"]`)) {
		t.Fatal("lost default choices")
	}
	for _, invalid := range []string{`["missing"]`, `["garlic","garlic"]`} {
		bad := bytes.Replace(withDefault, []byte(`"default_option_ids":["garlic"]`), []byte(`"default_option_ids":`+invalid), 1)
		if _, err := validate(bad); err == nil {
			t.Fatal("accepted invalid default choices")
		}
	}
}
