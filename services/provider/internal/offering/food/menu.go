// Package food implements the initial menu model, not a universal catalog.
package food

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"ocp.local/platform/internal/offering"
)

var Model = offering.Model{ID: "ocp.food.menu", Version: "1.0.0"}

type menu struct {
	Currency         string `json:"currency"`
	AvailabilityMode string `json:"availability_mode"`
	Items            []item `json:"items"`
}
type item struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Variants  []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Price int64  `json:"price_minor"`
	} `json:"variants"`
	Groups []group `json:"option_groups"`
}
type group struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Min     int    `json:"min_selected"`
	Max     int    `json:"max_selected"`
	Options []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Price int64  `json:"price_minor"`
	} `json:"options"`
}

func Validator(schema *openapi3.Schema) offering.Validator {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var document any
		if err := json.Unmarshal(raw, &document); err != nil {
			return nil, offering.ErrInvalid
		}
		if err := schema.VisitJSON(document); err != nil {
			return nil, offering.ErrInvalid
		}
		var parsed menu
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, offering.ErrInvalid
		}
		items := map[string]bool{}
		for _, item := range parsed.Items {
			if items[item.ID] || strings.TrimSpace(item.Name) == "" {
				return nil, offering.ErrInvalid
			}
			items[item.ID] = true
			variants := map[string]bool{}
			for _, variant := range item.Variants {
				if variants[variant.ID] || strings.TrimSpace(variant.Name) == "" {
					return nil, offering.ErrInvalid
				}
				variants[variant.ID] = true
			}
			groups := map[string]bool{}
			for _, group := range item.Groups {
				if groups[group.ID] || strings.TrimSpace(group.Name) == "" || group.Min > group.Max || group.Max > len(group.Options) {
					return nil, offering.ErrInvalid
				}
				groups[group.ID] = true
				choices := map[string]bool{}
				for _, option := range group.Options {
					if choices[option.ID] || strings.TrimSpace(option.Name) == "" {
						return nil, offering.ErrInvalid
					}
					choices[option.ID] = true
				}
			}
		}
		// Shape/semantic validation precedes canonical JSON. Map keys are sorted and
		// numeric spellings normalized; bounds keep all supported integers exact.
		canonical, err := json.Marshal(document)
		if err != nil {
			return nil, fmt.Errorf("canonical menu: %w", err)
		}
		return canonical, nil
	}
}
