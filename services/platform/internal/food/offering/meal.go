package food

import (
	"encoding/json"
	"ocp.local/platform/internal/core/offering"
	"slices"
	"strings"
	"time"
)

type addonChoice struct {
	ItemID    string `json:"item_id"`
	VariantID string `json:"variant_id"`
}
type mealDeal struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Translations map[string]string `json:"translations,omitempty"`
	MainItemIDs  []string          `json:"main_item_ids"`
	Surcharge    int64             `json:"surcharge_minor"`
	AddonChoices []addonChoice     `json:"addon_choices"`
}
type SelectedMealDeal struct {
	DealID         string `json:"deal_id"`
	AddonItemID    string `json:"addon_item_id"`
	AddonVariantID string `json:"addon_variant_id"`
}
type QuotedMealDeal struct {
	SelectedMealDeal
	Name             string `json:"name"`
	AddonItemName    string `json:"addon_item_name"`
	AddonVariantName string `json:"addon_variant_name"`
	AddonQuantity    int    `json:"addon_quantity"`
	Surcharge        int64  `json:"surcharge_minor"`
}

func validateDeals(m menu) error {
	items := map[string]item{}
	for _, item := range m.Items {
		items[item.ID] = item
	}
	seen := map[string]bool{}
	for _, deal := range m.MealDeals {
		if seen[deal.ID] || strings.TrimSpace(deal.Name) == "" {
			return offering.ErrInvalid
		}
		seen[deal.ID] = true
		for _, id := range deal.MainItemIDs {
			if _, ok := items[id]; !ok {
				return offering.ErrInvalid
			}
		}
		for _, choice := range deal.AddonChoices {
			item, ok := items[choice.ItemID]
			if !ok {
				return offering.ErrInvalid
			}
			found := false
			for _, v := range item.Variants {
				if v.ID == choice.VariantID {
					found = true
				}
			}
			if !found {
				return offering.ErrInvalid
			}
			for _, g := range item.Groups {
				if g.Min > 0 {
					return offering.ErrInvalid
				}
			}
		}
	}
	return nil
}

func quoteMeal(m menu, line SelectionLine) (*QuotedMealDeal, error) {
	if line.MealDeal == nil {
		return nil, nil
	}
	for _, deal := range m.MealDeals {
		if deal.ID != line.MealDeal.DealID || !slices.Contains(deal.MainItemIDs, line.ItemID) {
			continue
		}
		for _, choice := range deal.AddonChoices {
			if choice.ItemID != line.MealDeal.AddonItemID || choice.VariantID != line.MealDeal.AddonVariantID {
				continue
			}
			for _, item := range m.Items {
				if item.ID != choice.ItemID || !item.Available {
					continue
				}
				for _, group := range item.Groups {
					if group.Min > 0 {
						return nil, ErrSelection
					}
				}
				for _, variant := range item.Variants {
					if variant.ID == choice.VariantID {
						if deal.Surcharge < 0 || deal.Surcharge > 1000000000 {
							return nil, ErrSelection
						}
						return &QuotedMealDeal{SelectedMealDeal: *line.MealDeal, Name: deal.Name, AddonItemName: item.Name, AddonVariantName: variant.Name, AddonQuantity: line.Quantity, Surcharge: deal.Surcharge}, nil
					}
				}
			}
		}
	}
	return nil, ErrSelection
}

// PriceModel is the sole installed Food model; scheduling has been retired.
func PriceModel(model offering.Model, raw json.RawMessage, lines []SelectionLine, locale string, now time.Time) ([]QuotedLine, int64, time.Time, error) {
	if model != Model {
		return nil, 0, time.Time{}, ErrSelection
	}
	q, total, err := PriceLocalized(raw, lines, locale)
	return q, total, now.Add(10 * time.Minute), err
}
