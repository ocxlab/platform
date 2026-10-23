package devfixtures

import (
	"encoding/json"
	"testing"
)

func TestDemoTranslationsCoverMenuAndPreserveEdits(t *testing.T) {
	menu, _, err := Restaurant()
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(menu)
	var doc map[string]any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	if !TranslateDemoNames(doc) || TranslateDemoNames(doc) {
		t.Fatal("enrichment must change once")
	}
	for _, raw := range doc["items"].([]any) {
		item := raw.(map[string]any)
		if item["translations"] == nil {
			t.Fatalf("missing translations: %v", item["name"])
		}
	}
	renamed := map[string]any{"name": "My dish", "translations": map[string]string{"en": "Custom"}}
	if TranslateDemoNames(renamed) {
		t.Fatal("overwrote merchant translations")
	}
}
