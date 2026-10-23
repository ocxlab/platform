package agenttools

import (
	"encoding/json"
	"testing"
)

func TestItemPatchPreservesUnspecifiedFields(t *testing.T) {
	raw := json.RawMessage(`{"currency":"EUR","items":[{"id":"x","name":"Old","translations":{"de":"Alt"},"image_id":"image","variants":[{"id":"s","price_minor":700}],"option_groups":[]}],"meal_deals":[{"id":"deal"}]}`)
	name := "New"
	empty := ""
	updated, err := patchMenuItem(raw, saveItemInput{Item: "x", ExpectedRevision: 1, Fields: itemPatch{Name: &name, Image: &empty}})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(updated, &doc)
	if string(doc["meal_deals"]) != `[{"id":"deal"}]` {
		t.Fatal(string(updated))
	}
	var items []map[string]any
	_ = json.Unmarshal(doc["items"], &items)
	if items[0]["name"] != "New" || items[0]["image_id"] != nil || items[0]["translations"].(map[string]any)["de"] != "Alt" || len(items[0]["variants"].([]any)) != 1 {
		t.Fatal(items)
	}
	for _, input := range []saveItemInput{{Item: "missing", ExpectedRevision: 1, Fields: itemPatch{Name: &name}}, {Item: "x", Create: true, Fields: itemPatch{Name: &name}}, {Item: "x", Fields: itemPatch{}}, {Item: "", Fields: itemPatch{Name: &name}}, {Item: "x", ExpectedRevision: -1, Fields: itemPatch{Name: &name}}} {
		if _, err := patchMenuItem(raw, input); err == nil {
			t.Fatalf("accepted invalid %+v", input)
		}
	}
}

func TestCategoryAssignmentDoesNotMutateOtherItems(t *testing.T) {
	raw := json.RawMessage(`{"items":[{"id":"a","category_id":"old","name":"A"},{"id":"b","category_id":"old","name":"B"}],"other":7}`)
	updated, err := organizeCategories(raw, categoriesInput{Categories: []menuCategory{}, Assignments: map[string]string{"a": ""}})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Items []map[string]any
		Other int
	}
	_ = json.Unmarshal(updated, &doc)
	if doc.Other != 7 || doc.Items[0]["category_id"] != nil || doc.Items[1]["category_id"] != "old" {
		t.Fatal(doc)
	}
	if _, err := organizeCategories(raw, categoriesInput{Assignments: map[string]string{"missing": "x"}}); err == nil {
		t.Fatal("unknown item accepted")
	}
}
