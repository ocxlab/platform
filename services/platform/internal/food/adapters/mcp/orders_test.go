package agenttools

import (
	"encoding/json"
	foodstore "ocp.local/platform/internal/food/persistence"
	"testing"
)

func TestOrderProgressiveDisclosure(t *testing.T) {
	lines := make([]map[string]any, 21)
	for i := range lines {
		lines[i] = map[string]any{"quantity": i + 1}
	}
	raw, _ := json.Marshal(lines)
	order := foodstore.FoodOrder{Revision: 3, Terms: foodstore.FoodQuote{Lines: raw, Note: "Order note"}, History: []foodstore.FoodOrderEvent{{Sequence: 1, Event: "placed"}}}
	first, err := orderDetails(order, orderReadInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first["items"].([]json.RawMessage)) != 20 || first["next_offset"] != 20 || first["history"] != nil || first["note"] != "Order note" {
		t.Fatal(first)
	}
	last, err := orderDetails(order, orderReadInput{Offset: 20, ExpectedRevision: 3, History: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(last["items"].([]json.RawMessage)) != 1 || last["next_offset"] != nil || len(last["history"].([]foodstore.FoodOrderEvent)) != 1 {
		t.Fatal(last)
	}
	for _, in := range []orderReadInput{{Offset: -1}, {ExpectedRevision: -1}, {Offset: 1}, {ExpectedRevision: 2}, {Offset: 22, ExpectedRevision: 3}} {
		if _, err := orderDetails(order, in); err == nil {
			t.Fatalf("invalid continuation %+v", in)
		}
	}
}
