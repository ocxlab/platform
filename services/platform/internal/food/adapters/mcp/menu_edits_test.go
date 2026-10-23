package agenttools

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"ocp.local/platform/internal/core/application"
	"ocp.local/platform/internal/core/persistence"
)

const priceMenu = `{"currency":"EUR","availability_mode":"manual","items":[{"id":"pizza","name":"بيتزا","available":true,"variants":[{"id":"small","name":"Small","price_minor":700},{"id":"large","name":"Large","price_minor":950}],"option_groups":[{"id":"extras","name":"Extras","min_selected":0,"max_selected":2,"options":[{"id":"cheese","name":"Käse","price_minor":100},{"id":"garlic","name":"Garlic","price_minor":0}]}]},{"id":"bread","available":false,"variants":[{"id":"normal","price_minor":200}]}]}`

func TestEditItemPrices(t *testing.T) {
	in := pricesInput{Item: "pizza", ExpectedRevision: 1, Changes: []itemPriceChange{{Variant: "large", Price: 1000}, {Group: "extras", Option: "cheese", Price: 0}, {Group: "extras", Option: "garlic", Price: 50}}}
	raw, err := editItemPrices(json.RawMessage(priceMenu), in)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := menuChanges(json.RawMessage(priceMenu), raw)
	if err != nil || len(changes) != 3 {
		t.Fatalf("changes=%v err=%v", changes, err)
	}
	want := []menuChange{
		{Path: "/items/0/option_groups/0/options/0/price_minor", Before: json.RawMessage(`100`), After: json.RawMessage(`0`)},
		{Path: "/items/0/option_groups/0/options/1/price_minor", Before: json.RawMessage(`0`), After: json.RawMessage(`50`)},
		{Path: "/items/0/variants/1/price_minor", Before: json.RawMessage(`950`), After: json.RawMessage(`1000`)},
	}
	for i := range want {
		want[i].BeforeItem = &changeItem{ID: "pizza", Name: "بيتزا"}
		want[i].AfterItem = &changeItem{ID: "pizza", Name: "بيتزا"}
	}
	if !reflect.DeepEqual(changes, want) {
		t.Fatalf("unexpected changes: %+v", changes)
	}
	for _, changes := range [][]itemPriceChange{
		nil, {}, {{Variant: "large", Price: -1}}, {{Variant: "large", Price: 1_000_000_001}},
		{{Price: 0}}, {{Variant: "large", Group: "extras", Option: "cheese", Price: 1}},
		{{Group: "extras", Price: 1}}, {{Option: "cheese", Price: 1}},
		{{Variant: "large", Price: 1}, {Variant: "large", Price: 2}},
		{{Group: "extras", Option: "cheese", Price: 1}, {Group: "extras", Option: "cheese", Price: 2}},
		{{Variant: strings.Repeat("a", 65), Price: 1}},
		make([]itemPriceChange, 21),
	} {
		in.Changes = changes
		if _, err := editItemPrices(json.RawMessage(priceMenu), in); !errors.Is(err, commerce.ErrInvalid) {
			t.Fatalf("invalid change accepted: %+v %v", changes, err)
		}
	}
	for _, changes := range [][]itemPriceChange{
		{{Variant: "absent", Price: 1}}, {{Group: "absent", Option: "cheese", Price: 1}}, {{Group: "extras", Option: "absent", Price: 1}},
		{{Variant: "large", Price: 1}, {Group: "extras", Option: "absent", Price: 1}},
	} {
		in.Changes = changes
		if raw, err := editItemPrices(json.RawMessage(priceMenu), in); raw != nil || !errors.Is(err, persistence.ErrNotFound) {
			t.Fatalf("missing target partially succeeded: %s %v", raw, err)
		}
	}
	in.Changes = []itemPriceChange{{Variant: "large", Price: 1}}
	in.Item = "absent"
	if _, err := editItemPrices(json.RawMessage(priceMenu), in); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestMenuChanges(t *testing.T) {
	before := json.RawMessage(`{"items":[{"id":"x","name":"Old"}],"a/b~c":[],"n":1000000000}`)
	after := json.RawMessage(`{"n":1000000000,"items":[{"name":"New","id":"x"}],"a/b~c":["hello"],"added":true}`)
	changes, err := menuChanges(before, after)
	if err != nil || len(changes) != 4 {
		t.Fatalf("%+v %v", changes, err)
	}
	if changes[1].Path != "/a~1b~0c" || changes[1].After != nil || changes[2].Path != "/a~1b~0c/0" || changes[2].Before != nil || changes[3].Path != "/items/0/name" {
		t.Fatalf("bad pointers/presence: %+v", changes)
	}
	empty, err := menuChanges(before, before)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatal("empty diff must be []")
	}
	if _, err := menuChanges(json.RawMessage(`{broken`), after); err == nil {
		t.Fatal("invalid document accepted")
	}
	// Large additions stay scalar, not a single unbounded item/menu echo.
	changes, err = menuChanges(json.RawMessage(`{"items":[]}`), json.RawMessage(priceMenu))
	if err != nil || len(changes) < 20 {
		t.Fatal("missing leaf-level additions")
	}
	for _, change := range changes {
		if len(change.After) > 200 {
			t.Fatal("unbounded nested value")
		}
	}
}

func TestExtraVariantPrices(t *testing.T) {
	in := pricesInput{Item: "pizza", ExpectedRevision: 1, Changes: []itemPriceChange{
		{Group: "extras", Option: "cheese", ExtraVariant: "large", Price: 200},
		{Group: "extras", Option: "cheese", ExtraVariant: "small", Price: 0},
		{Group: "extras", Option: "cheese", Price: 150},
	}}
	raw, err := editItemPrices(json.RawMessage(priceMenu), in)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Items []menuItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	option := doc.Items[0].Groups[0].Options[0]
	if option.Price != 150 || !reflect.DeepEqual(option.VariantPrices, map[string]int64{"large": 200, "small": 0}) {
		t.Fatalf("lost base or size-specific surcharge: %+v", option)
	}
	// Updating an existing override preserves the other sizes and all other fields.
	in.Changes = []itemPriceChange{{Group: "extras", Option: "cheese", ExtraVariant: "large", Price: 250}}
	updated, err := editItemPrices(raw, in)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := menuChanges(raw, updated)
	if err != nil || len(changes) != 1 || changes[0].Path != "/items/0/option_groups/0/options/0/variant_prices/large" || string(changes[0].After) != "250" {
		t.Fatalf("changed unrelated fields: %+v %v", changes, err)
	}
	for _, tc := range []struct {
		changes []itemPriceChange
		want    error
	}{
		{[]itemPriceChange{{ExtraVariant: "large", Price: 1}}, commerce.ErrInvalid},
		{[]itemPriceChange{{Variant: "large", ExtraVariant: "large", Price: 1}}, commerce.ErrInvalid},
		{[]itemPriceChange{{Group: "extras", Option: "cheese", ExtraVariant: strings.Repeat("x", 65), Price: 1}}, commerce.ErrInvalid},
		{[]itemPriceChange{{Group: "extras", Option: "cheese", ExtraVariant: "large", Price: 1}, {Group: "extras", Option: "cheese", ExtraVariant: "large", Price: 2}}, commerce.ErrInvalid},
		{[]itemPriceChange{{Group: "extras", Option: "cheese", ExtraVariant: "absent", Price: 1}}, persistence.ErrNotFound},
		{[]itemPriceChange{{Variant: "large", Price: 1}, {Group: "extras", Option: "cheese", ExtraVariant: "absent", Price: 1}}, persistence.ErrNotFound},
	} {
		in.Changes = tc.changes
		out, err := editItemPrices(raw, in)
		if out != nil || !errors.Is(err, tc.want) {
			t.Fatalf("invalid override succeeded: %s %v", out, err)
		}
	}
}
