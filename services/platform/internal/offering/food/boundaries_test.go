package food

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestPriceBoundaries(t *testing.T) {
	base := SelectionLine{ItemID: "meal", VariantID: "regular", Quantity: 1}
	menuAt := func(price int64, extra int64) []byte {
		return []byte(fmt.Sprintf(`{"currency":"EUR","items":[{"id":"meal","name":"Meal","available":true,"variants":[{"id":"regular","name":"Regular","price_minor":%d}],"option_groups":[{"id":"extra","name":"Extra","min_selected":0,"max_selected":1,"options":[{"id":"cheese","name":"Cheese","price_minor":%d}]}]}]}`, price, extra))
	}
	for _, tc := range []struct {
		name         string
		price, extra int64
		quantity     int
		choose       bool
		want         int64
		valid        bool
	}{
		{"free", 0, 0, 1, false, 0, true},
		{"max-unit", MaxTotal, 0, 1, false, MaxTotal, true},
		{"max-plus-free-extra", MaxTotal, 0, 1, true, MaxTotal, true},
		{"negative-unit", -1, 0, 1, false, 0, false},
		{"too-large-unit", MaxTotal + 1, 0, 1, false, 0, false},
		{"negative-extra", 100, -1, 1, true, 0, false},
		{"exact-extra-limit", 1, MaxTotal - 1, 1, true, MaxTotal, true},
		{"extra-overflow", 1, MaxTotal, 1, true, 0, false},
		{"quantity-max", 1, 0, 99, false, 99, true},
		{"quantity-zero", 1, 0, 0, false, 0, false},
		{"quantity-over", 1, 0, 100, false, 0, false},
		{"exact-multiplication", MaxTotal / 2, 0, 2, false, MaxTotal, true},
		{"multiplication-overflow", MaxTotal/2 + 1, 0, 2, false, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := base
			line.Quantity = tc.quantity
			if tc.choose {
				line.Options = []SelectedOption{{"extra", "cheese"}}
			}
			_, got, err := Price(menuAt(tc.price, tc.extra), []SelectionLine{line})
			if tc.valid && (err != nil || got != tc.want) {
				t.Fatalf("got %d %v", got, err)
			}
			if !tc.valid && err == nil {
				t.Fatal("invalid boundary accepted")
			}
		})
	}
	for _, n := range []int{0, 1, 20, 21} {
		lines := make([]SelectionLine, n)
		for i := range lines {
			lines[i] = base
		}
		_, total, err := Price(menuAt(1, 0), lines)
		if n >= 1 && n <= 20 {
			if err != nil || total != int64(n) {
				t.Fatalf("line boundary %d: %d %v", n, total, err)
			}
		} else if err == nil {
			t.Fatalf("accepted %d lines", n)
		}
	}
	_, total, err := Price(menuAt(MaxTotal/2, 0), []SelectionLine{base, base})
	if err != nil || total != MaxTotal {
		t.Fatal("exact accumulated total rejected")
	}
	if _, _, err := Price(menuAt(MaxTotal/2+1, 0), []SelectionLine{base, base}); err == nil {
		t.Fatal("accumulated overflow accepted")
	}
	if _, err := Project([]byte(`{`)); err == nil {
		t.Fatal("invalid JSON projection accepted")
	}
}

func TestPriceMaximumSelectedOptions(t *testing.T) {
	// Exercise the pricing guard independently of HTTP schema validation.
	var doc menu
	if err := json.Unmarshal([]byte(publicMenu), &doc); err != nil {
		t.Fatal(err)
	}
	doc.Items[0].Groups = nil
	line := SelectionLine{ItemID: "meal", VariantID: "regular", Quantity: 1}
	for g := 0; g < 20; g++ {
		var group group
		group.ID = fmt.Sprintf("g%d", g)
		group.Name = group.ID
		group.Max = 30
		for o := 0; o < 30; o++ {
			choice := struct {
				VariantPrices map[string]int64  `json:"variant_prices,omitempty"`
				Translations  map[string]string `json:"translations,omitempty"`
				ID            string            `json:"id"`
				Name          string            `json:"name"`
				Price         int64             `json:"price_minor"`
			}{nil, nil, fmt.Sprintf("o%d", o), "Extra", 0}
			group.Options = append(group.Options, choice)
			line.Options = append(line.Options, SelectedOption{group.ID, choice.ID})
		}
		doc.Items[0].Groups = append(doc.Items[0].Groups, group)
	}
	raw, _ := json.Marshal(doc)
	if _, total, err := Price(raw, []SelectionLine{line}); err != nil || total != 700 {
		t.Fatalf("600 choices rejected: %d %v", total, err)
	}
	line.Options = append(line.Options, SelectedOption{"another", "another"})
	if _, _, err := Price(raw, []SelectionLine{line}); err == nil {
		t.Fatal("over 600 choices accepted")
	}
}
