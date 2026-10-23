// Package food implements the initial menu model, not a universal catalog.
package food

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"ocp.local/platform/internal/core/offering"
)

var Model = offering.FoodMenu

type menu struct {
	MealDeals        []mealDeal `json:"meal_deals,omitempty"`
	Currency         string     `json:"currency"`
	AvailabilityMode string     `json:"availability_mode"`
	Items            []item     `json:"items"`
	Categories       []category `json:"categories,omitempty"`
}
type category struct {
	Translations map[string]string `json:"translations,omitempty"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
}
type item struct {
	Translations map[string]string `json:"translations,omitempty"`
	CategoryID   string            `json:"category_id,omitempty"`
	ImageID      string            `json:"image_id,omitempty"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Available    bool              `json:"available"`
	Variants     []struct {
		Translations map[string]string `json:"translations,omitempty"`
		ID           string            `json:"id"`
		Name         string            `json:"name"`
		Price        int64             `json:"price_minor"`
	} `json:"variants"`
	Groups []group `json:"option_groups"`
}
type group struct {
	Defaults     []string          `json:"default_option_ids,omitempty"`
	Translations map[string]string `json:"translations,omitempty"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Min          int               `json:"min_selected"`
	Max          int               `json:"max_selected"`
	Options      []struct {
		VariantPrices map[string]int64  `json:"variant_prices,omitempty"`
		Translations  map[string]string `json:"translations,omitempty"`
		ID            string            `json:"id"`
		Name          string            `json:"name"`
		Price         int64             `json:"price_minor"`
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
		categories := map[string]bool{}
		for _, category := range parsed.Categories {
			if categories[category.ID] || strings.TrimSpace(category.Name) == "" {
				return nil, offering.ErrInvalid
			}
			categories[category.ID] = true
		}
		items := map[string]bool{}
		for _, item := range parsed.Items {
			if item.CategoryID != "" && !categories[item.CategoryID] {
				return nil, offering.ErrInvalid
			}
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
					for variant := range option.VariantPrices {
						if !variants[variant] {
							return nil, offering.ErrInvalid
						}
					}
				}
				if len(group.Defaults) > 0 {
					if len(group.Defaults) < group.Min || len(group.Defaults) > group.Max {
						return nil, offering.ErrInvalid
					}
					seen := map[string]bool{}
					for _, id := range group.Defaults {
						if !choices[id] || seen[id] {
							return nil, offering.ErrInvalid
						}
						seen[id] = true
					}
				}
			}
		}
		if err := validateDeals(parsed); err != nil {
			return nil, err
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
