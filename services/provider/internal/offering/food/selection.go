package food

import (
	"encoding/json"
	"errors"
	"unicode/utf8"
)

var ErrSelection = errors.New("invalid or unavailable selection")

const MaxTotal int64 = 1000000000000

// Project deliberately enumerates public fields through the installed model.
// Adding a private draft field must never automatically add it to publication.
func Project(raw json.RawMessage) (json.RawMessage, error) {
	var value menu
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

type SelectedOption struct {
	GroupID  string `json:"group_id"`
	OptionID string `json:"option_id"`
}
type SelectionLine struct {
	Note      string           `json:"note,omitempty"`
	ItemID    string           `json:"item_id"`
	VariantID string           `json:"variant_id"`
	Quantity  int              `json:"quantity"`
	Options   []SelectedOption `json:"options"`
}
type QuotedLine struct {
	PriceExplanation *PriceExplanation `json:"price_explanation,omitempty"`
	Selection        SelectionLine     `json:"selection"`
	ItemName         string            `json:"item_name"`
	VariantName      string            `json:"variant_name"`
	UnitPrice        int64             `json:"unit_price_minor"`
	Total            int64             `json:"total_minor"`
	SelectedOptions  []QuotedOption    `json:"selected_options"`
}

type QuotedOption struct {
	Price      *int64 `json:"price_minor,omitempty"`
	GroupID    string `json:"group_id"`
	OptionID   string `json:"option_id"`
	GroupName  string `json:"group_name"`
	OptionName string `json:"option_name"`
}

// Price uses only the published model and selection references. Neither UI totals
// nor caller-supplied prices are inputs. It does not reserve or accept an order.
func Price(raw json.RawMessage, lines []SelectionLine) ([]QuotedLine, int64, error) {
	var value menu
	if err := json.Unmarshal(raw, &value); err != nil || value.Currency != "EUR" {
		return nil, 0, ErrSelection
	}
	if len(lines) < 1 || len(lines) > 20 {
		return nil, 0, ErrSelection
	}
	quoted := make([]QuotedLine, 0, len(lines))
	var total int64
	for _, line := range lines {
		if line.Quantity < 1 || line.Quantity > 99 || len(line.Options) > 600 || utf8.RuneCountInString(line.Note) > 500 {
			return nil, 0, ErrSelection
		}
		var selected *item
		for i := range value.Items {
			if value.Items[i].ID == line.ItemID {
				selected = &value.Items[i]
				break
			}
		}
		if selected == nil || !selected.Available {
			return nil, 0, ErrSelection
		}
		unit := int64(-1)
		name := ""
		for _, variant := range selected.Variants {
			if variant.ID == line.VariantID {
				unit = variant.Price
				name = variant.Name
				break
			}
		}
		if unit < 0 || unit > MaxTotal {
			return nil, 0, ErrSelection
		}
		seen := map[SelectedOption]bool{}
		descriptions := make([]QuotedOption, 0, len(line.Options))
		counts := map[string]int{}
		for _, option := range line.Options {
			if seen[option] {
				return nil, 0, ErrSelection
			}
			seen[option] = true
			found := false
			for _, group := range selected.Groups {
				if group.ID == option.GroupID {
					for _, choice := range group.Options {
						if choice.ID == option.OptionID {
							if choice.Price < 0 || choice.Price > MaxTotal-unit {
								return nil, 0, ErrSelection
							}
							unit += choice.Price
							counts[group.ID]++
							descriptions = append(descriptions, QuotedOption{GroupID: group.ID, OptionID: choice.ID, GroupName: group.Name, OptionName: choice.Name})
							found = true
							break
						}
					}
				}
			}
			if !found {
				return nil, 0, ErrSelection
			}
		}
		for _, group := range selected.Groups {
			if counts[group.ID] < group.Min || counts[group.ID] > group.Max {
				return nil, 0, ErrSelection
			}
		}
		if unit > MaxTotal/int64(line.Quantity) {
			return nil, 0, ErrSelection
		}
		lineTotal := unit * int64(line.Quantity)
		if lineTotal > MaxTotal-total {
			return nil, 0, ErrSelection
		}
		total += lineTotal
		quoted = append(quoted, QuotedLine{Selection: line, ItemName: selected.Name, VariantName: name, UnitPrice: unit, Total: lineTotal, SelectedOptions: descriptions})
	}
	return quoted, total, nil
}

// PriceLocalized retains display labels in the requested supported language.
// Identifiers, availability and prices remain exactly those of the publication.
func PriceLocalized(raw json.RawMessage, lines []SelectionLine, locale string) ([]QuotedLine, int64, error) {
	var value menu
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, 0, ErrSelection
	}
	name := func(base string, translations map[string]string) string {
		if (locale == "de" || locale == "en" || locale == "ar") && translations[locale] != "" {
			return translations[locale]
		}
		return base
	}
	for i := range value.Items {
		item := &value.Items[i]
		item.Name = name(item.Name, item.Translations)
		for j := range item.Variants {
			v := &item.Variants[j]
			v.Name = name(v.Name, v.Translations)
		}
		for j := range item.Groups {
			g := &item.Groups[j]
			g.Name = name(g.Name, g.Translations)
			for k := range g.Options {
				o := &g.Options[k]
				o.Name = name(o.Name, o.Translations)
			}
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, 0, err
	}
	return Price(encoded, lines)
}
