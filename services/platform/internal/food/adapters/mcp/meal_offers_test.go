package agenttools

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"ocp.local/platform/internal/core/application"
	"ocp.local/platform/internal/core/offering"
	"ocp.local/platform/internal/core/persistence"
)

func mealFixture(t *testing.T) json.RawMessage {
	t.Helper()
	choices := make([]mealChoice, 0, 21)
	variants := make([]menuPrice, 0, 21)
	for i := range 21 {
		id := fmt.Sprintf("size-%d", i)
		choices = append(choices, mealChoice{"drink", id})
		variants = append(variants, menuPrice{id, id, 300})
	}
	doc := mealMenu{Currency: "EUR", Items: []menuItem{{ID: "main"}, {ID: "other"}, {ID: "drink", Name: "Drink", Available: false, Variants: variants}}}
	for i := range 21 {
		doc.Offers = append(doc.Offers, mealOffer{ID: fmt.Sprintf("meal-%d", i), Name: "Lunch", Translations: map[string]string{"de": "Mittag"}, MainItemIDs: []string{"main"}, Surcharge: 100, AddonChoices: choices})
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestMealOfferPages(t *testing.T) {
	raw := mealFixture(t)
	for _, tc := range []struct {
		in    mealReadInput
		count int
		next  bool
	}{
		{mealReadInput{}, 20, true},
		{mealReadInput{Revision: 1, Offset: 20}, 1, false},
		{mealReadInput{Revision: 1, Offset: 21}, 0, false},
		{mealReadInput{Item: "other"}, 0, false},
		{mealReadInput{Item: "main"}, 20, true},
	} {
		out, err := mealOfferPage(raw, tc.in)
		if err != nil {
			t.Fatal(err)
		}
		if len(out["offers"].([]any)) != tc.count || (out["next_offset"] != nil) != tc.next || out["currency"] != "EUR" {
			t.Fatalf("wrong page: %v", out)
		}
		wire, _ := json.Marshal(out)
		if strings.Contains(string(wire), "addon_choices") || strings.Contains(string(wire), "main_item_ids") {
			t.Fatal("summary leaks choice lists")
		}
	}
	for _, tc := range []struct {
		offset, count int
		next          bool
	}{{0, 20, true}, {20, 1, false}, {21, 0, false}} {
		out, err := mealOfferPage(raw, mealReadInput{Offer: "meal-0", Revision: 1, Offset: tc.offset})
		if err != nil {
			t.Fatal(err)
		}
		detail := out["offer"].(map[string]any)
		choices := detail["addon_choices"].([]any)
		if len(choices) != tc.count || detail["addon_choice_count"] != 21 || detail["surcharge_minor"] != int64(100) || !reflect.DeepEqual(detail["main_item_ids"], []string{"main"}) || (out["next_offset"] != nil) != tc.next {
			t.Fatalf("bad details: %v", out)
		}
		if tc.count > 0 {
			choice := choices[0].(map[string]any)
			if choice["available"] != false || choice["item_name"] != "Drink" || choice["variant_id"] != fmt.Sprintf("size-%d", tc.offset) || choice["variant_name"] != choice["variant_id"] {
				t.Fatalf("bad choice: %v", choice)
			}
		}
	}
	for _, tc := range []struct {
		in   mealReadInput
		want error
	}{
		{mealReadInput{Revision: -1}, commerce.ErrInvalid},
		{mealReadInput{Offset: -1}, commerce.ErrInvalid},
		{mealReadInput{Offset: 1}, commerce.ErrInvalid},
		{mealReadInput{Offer: "meal-0"}, commerce.ErrInvalid},
		{mealReadInput{Item: "main", Offer: "meal-0", Revision: 1}, commerce.ErrInvalid},
		{mealReadInput{Item: strings.Repeat("i", 65)}, commerce.ErrInvalid},
		{mealReadInput{Offer: strings.Repeat("i", 65), Revision: 1}, commerce.ErrInvalid},
		{mealReadInput{Revision: 1, Offset: 22}, commerce.ErrInvalid},
		{mealReadInput{Offer: "meal-0", Revision: 1, Offset: 22}, commerce.ErrInvalid},
		{mealReadInput{Offer: "missing", Revision: 1}, persistence.ErrNotFound},
		{mealReadInput{Item: "missing"}, persistence.ErrNotFound},
	} {
		if _, err := mealOfferPage(raw, tc.in); !errors.Is(err, tc.want) {
			t.Fatalf("%+v: %v", tc.in, err)
		}
	}
	if _, err := mealOfferPage(json.RawMessage(`{`), mealReadInput{}); !errors.Is(err, offering.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := mealOfferPage(json.RawMessage(`{"meal_deals":[{"id":"x","addon_choices":[{"item_id":"missing"}]}]}`), mealReadInput{Offer: "x", Revision: 1}); !errors.Is(err, offering.ErrInvalid) {
		t.Fatal(err)
	}
	empty, err := mealOfferPage(json.RawMessage(`{"currency":"EUR","items":[]}`), mealReadInput{})
	if err != nil || len(empty["offers"].([]any)) != 0 {
		t.Fatal(empty, err)
	}
}

func TestMealOfferEdits(t *testing.T) {
	raw := mealFixture(t)
	in := mealSaveInput{ExpectedRevision: 1, Offer: mealOffer{ID: "meal-0", Name: "New lunch", MainItemIDs: []string{"other"}, Surcharge: 0, AddonChoices: []mealChoice{{"drink", "size-0"}}}}
	edited, err := editMealOffer(raw, in)
	if err != nil {
		t.Fatal(err)
	}
	var before, after map[string]json.RawMessage
	_ = json.Unmarshal(raw, &before)
	_ = json.Unmarshal(edited, &after)
	if !reflect.DeepEqual(before["items"], after["items"]) || !reflect.DeepEqual(before["currency"], after["currency"]) {
		t.Fatal("changed other menu fields")
	}
	var a, b []map[string]json.RawMessage
	_ = json.Unmarshal(before["meal_deals"], &a)
	_ = json.Unmarshal(after["meal_deals"], &b)
	if !reflect.DeepEqual(a[1:], b[1:]) || !reflect.DeepEqual(a[0]["translations"], b[0]["translations"]) || string(b[0]["surcharge_minor"]) != "0" || rawString(b[0]["name"]) != "New lunch" || string(b[0]["main_item_ids"]) != `["other"]` || string(b[0]["addon_choices"]) != `[{"item_id":"drink","variant_id":"size-0"}]` {
		t.Fatalf("bad edit: %s", edited)
	}
	// Omitted translations preserve; explicit empty clears; supplied names replace.
	for _, translations := range []map[string]string{{}, {"en": "Lunch"}} {
		in.Offer.Translations = translations
		updated, err := editMealOffer(raw, in)
		if err != nil {
			t.Fatal(err)
		}
		var doc mealMenu
		_ = json.Unmarshal(updated, &doc)
		if !reflect.DeepEqual(doc.Offers[0].Translations, translations) {
			t.Fatal("translations not replaced")
		}
	}
	in.Offer.ID = "new-meal"
	if _, err := editMealOffer(raw, in); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatal(err)
	}
	in.Create = true
	for _, source := range []json.RawMessage{raw, json.RawMessage(`{"currency":"EUR","items":[]}`)} {
		created, err := editMealOffer(source, in)
		if err != nil || !strings.Contains(string(created), `"id":"new-meal"`) {
			t.Fatal(string(created), err)
		}
	}
	in.Offer.ID = "meal-0"
	if _, err := editMealOffer(raw, in); !errors.Is(err, commerce.ErrInvalid) {
		t.Fatal(err)
	}
	for _, source := range []string{`{`, `null`, `{"meal_deals":null}`, `{"meal_deals":{}}`} {
		if _, err := editMealOffer(json.RawMessage(source), in); !errors.Is(err, offering.ErrInvalid) {
			t.Fatalf("%s: %v", source, err)
		}
	}
	for _, mutate := range []func(*mealSaveInput){
		func(v *mealSaveInput) { v.ExpectedRevision = 0 },
		func(v *mealSaveInput) { v.ExpectedRevision = -1 },
		func(v *mealSaveInput) { v.Offer.MainItemIDs = nil },
		func(v *mealSaveInput) { v.Offer.MainItemIDs = make([]string, 201) },
		func(v *mealSaveInput) { v.Offer.AddonChoices = nil },
		func(v *mealSaveInput) { v.Offer.AddonChoices = make([]mealChoice, 201) },
	} {
		bad := in
		mutate(&bad)
		if _, err := editMealOffer(raw, bad); !errors.Is(err, commerce.ErrInvalid) {
			t.Fatal(err)
		}
	}
}
