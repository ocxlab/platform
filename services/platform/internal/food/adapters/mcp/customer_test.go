package agenttools

import (
	"encoding/json"
	food "ocp.local/platform/internal/food/offering"
	store "ocp.local/platform/internal/food/persistence"
	"testing"
)

func TestReviseBasket(t *testing.T) {
	old := food.SelectionLine{ItemID: "old", VariantID: "regular", Quantity: 2, Note: "retain", Options: []food.SelectedOption{}}
	data, _ := json.Marshal([]food.QuotedLine{{Selection: old}, {Selection: old}})
	q := store.FoodQuote{Lines: data, Note: "order note"}
	next := food.SelectionLine{ItemID: "new", Quantity: 1}
	lines, note, err := reviseBasket(q, basketReviseInput{Changes: []basketChange{{Index: 0, Remove: true}}, Add: []food.SelectionLine{next}})
	if err != nil || len(lines) != 2 || lines[0].Note != "retain" || lines[1].ItemID != "new" || note != "order note" {
		t.Fatal(lines, note, err)
	}
	for _, changes := range [][]basketChange{{{Index: -1, Remove: true}}, {{Index: 2, Remove: true}}, {{Index: 0}}, {{Index: 0, Remove: true, Line: &next}}, {{Index: 0, Remove: true}, {Index: 0, Remove: true}}} {
		if _, _, err := reviseBasket(q, basketReviseInput{Changes: changes}); err == nil {
			t.Fatal("accepted invalid changes", changes)
		}
	}
	clear := ""
	lines, note, err = reviseBasket(q, basketReviseInput{Changes: []basketChange{{Index: 0, Line: &next}}, Note: &clear})
	if err != nil || lines[0].ItemID != "new" || note != "" {
		t.Fatal(lines, note, err)
	}
}
