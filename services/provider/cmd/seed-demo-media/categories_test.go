package main

import (
	"encoding/json"
	"ocp.local/platform/internal/devfixtures"
	"testing"
)

func TestAttachCategoriesPreservesExistingEdits(t *testing.T) {
	fixture, _, err := devfixtures.Restaurant()
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"items": []any{
		map[string]any{"id": fixture.Items[0].ID, "name": fixture.Items[0].Name, "image_id": "keep-image", "variants": "keep-price"},
		map[string]any{"id": fixture.Items[1].ID, "name": "My renamed dish"},
	}}
	if !attachCategories(doc, fixture) {
		t.Fatal("expected enrichment")
	}
	items := doc["items"].([]any)
	first := items[0].(map[string]any)
	if first["category_id"] != fixture.Items[0].CategoryID || first["image_id"] != "keep-image" || first["variants"] != "keep-price" {
		t.Fatal(first)
	}
	if items[1].(map[string]any)["category_id"] != nil {
		t.Fatal("renamed dish changed")
	}
	before, _ := json.Marshal(doc)
	if attachCategories(doc, fixture) {
		t.Fatal("repeat must be a no-op")
	}
	after, _ := json.Marshal(doc)
	if string(before) != string(after) {
		t.Fatal("existing categories changed")
	}
	doc["categories"] = []any{}
	if attachCategories(doc, fixture) {
		t.Fatal("explicit empty categories must be preserved")
	}
}
