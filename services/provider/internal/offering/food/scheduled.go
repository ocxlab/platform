package food

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"
	_ "time/tzdata"

	"github.com/getkin/kin-openapi/openapi3"
	"ocp.local/platform/internal/offering"
)

var ModelV2 = offering.Model{ID: "ocp.food.menu", Version: "2.0.0"}

type Window struct {
	Weekdays   []int      `json:"weekdays"`
	Start      string     `json:"start"`
	End        string     `json:"end"`
	ValidFrom  *time.Time `json:"valid_from,omitempty"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
}
type scheduledOffer struct {
	ID        string   `json:"id"`
	ItemID    string   `json:"item_id"`
	VariantID string   `json:"variant_id"`
	Price     int64    `json:"price_minor"`
	Priority  int      `json:"priority"`
	Windows   []Window `json:"windows"`
}

// ValidatorV2 preserves V1's identifier, default, and selection invariants while
// giving the new fields their own closed schema and temporal semantics.
func ValidatorV2(schema, v1 *openapi3.Schema) offering.Validator {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var document any
		if json.Unmarshal(raw, &document) != nil || schema.VisitJSON(document) != nil {
			return nil, offering.ErrInvalid
		}
		var value menu
		if json.Unmarshal(raw, &value) != nil {
			return nil, offering.ErrInvalid
		}
		if _, err := scheduleZone(value.TimeZone); err != nil {
			return nil, offering.ErrInvalid
		}
		for _, item := range value.Items {
			variants := map[string]bool{}
			if !validWindows(item.AvailabilityWindows) {
				return nil, offering.ErrInvalid
			}
			for _, variant := range item.Variants {
				variants[variant.ID] = true
				if !validWindows(variant.AvailabilityWindows) {
					return nil, offering.ErrInvalid
				}
			}
			for _, group := range item.Groups {
				for _, option := range group.Options {
					for variant := range option.VariantPrices {
						if !variants[variant] {
							return nil, offering.ErrInvalid
						}
					}
				}
			}
		}
		seen := map[string]bool{}
		for _, offer := range value.Offers {
			if seen[offer.ID] || !validWindows(offer.Windows) {
				return nil, offering.ErrInvalid
			}
			seen[offer.ID] = true
			found := false
			for _, item := range value.Items {
				if item.ID == offer.ItemID {
					for _, variant := range item.Variants {
						if variant.ID == offer.VariantID {
							found = true
						}
					}
				}
			}
			if !found {
				return nil, offering.ErrInvalid
			}
		}
		base := v1Menu(value)
		encoded, _ := json.Marshal(base)
		if _, err := Validator(v1)(encoded); err != nil {
			return nil, err
		}
		return json.Marshal(document)
	}
}

func scheduleZone(name string) (*time.Location, error) {
	if name == "" || name == "Local" {
		return nil, offering.ErrInvalid
	}
	return time.LoadLocation(name)
}

func minute(s string) int {
	if len(s) != 5 || s[2] != ':' {
		return -1
	}
	h, e1 := strconv.Atoi(s[:2])
	m, e2 := strconv.Atoi(s[3:])
	if e1 != nil || e2 != nil || h < 0 || h > 24 || m < 0 || m > 59 || (h == 24 && m != 0) {
		return -1
	}
	return h*60 + m
}
func validWindows(windows []Window) bool {
	seen := map[string]bool{}
	for _, w := range windows {
		if minute(w.Start) < 0 || minute(w.Start) >= 1440 || minute(w.Start) >= minute(w.End) {
			return false
		}
		if w.ValidFrom != nil && w.ValidUntil != nil && !w.ValidFrom.Before(*w.ValidUntil) {
			return false
		}
		w.Weekdays = append([]int(nil), w.Weekdays...)
		sort.Ints(w.Weekdays)
		key, _ := json.Marshal(w)
		if seen[string(key)] {
			return false
		}
		seen[string(key)] = true
	}
	return true
}
func active(windows []Window, now time.Time, zone *time.Location) bool {
	local := now.In(zone)
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	m := local.Hour()*60 + local.Minute()
	for _, w := range windows {
		if w.ValidFrom != nil && now.Before(*w.ValidFrom) || w.ValidUntil != nil && !now.Before(*w.ValidUntil) {
			continue
		}
		if m < minute(w.Start) || m >= minute(w.End) {
			continue
		}
		for _, day := range w.Weekdays {
			if day == weekday {
				return true
			}
		}
	}
	return false
}
func v1Menu(value menu) menu {
	value.TimeZone = ""
	value.Offers = nil
	for i := range value.Items {
		value.Items[i].AvailabilityWindows = nil
		for j := range value.Items[i].Variants {
			value.Items[i].Variants[j].AvailabilityWindows = nil
		}
		for j := range value.Items[i].Groups {
			for k := range value.Items[i].Groups[j].Options {
				value.Items[i].Groups[j].Options[k].VariantPrices = nil
			}
		}
	}
	return value
}

type PriceExplanation struct {
	Base          int64     `json:"base_price_minor"`
	OfferID       string    `json:"offer_id,omitempty"`
	EffectiveBase int64     `json:"effective_base_price_minor"`
	Extras        int64     `json:"extras_price_minor"`
	EvaluatedAt   time.Time `json:"evaluated_at"`
}

// PriceModel evaluates an exact registered model at the server's instant. The
// returned deadline caps the ordinary quote lifetime at relevant schedule edges.
func PriceModel(model offering.Model, raw json.RawMessage, lines []SelectionLine, locale string, now time.Time) ([]QuotedLine, int64, time.Time, error) {
	deadline := now.Add(10 * time.Minute)
	if model == Model {
		q, total, err := PriceLocalized(raw, lines, locale)
		return q, total, deadline, err
	}
	if model != ModelV2 {
		return nil, 0, time.Time{}, ErrSelection
	}
	if len(lines) < 1 || len(lines) > 20 {
		return nil, 0, time.Time{}, ErrSelection
	}
	var value menu
	if json.Unmarshal(raw, &value) != nil {
		return nil, 0, time.Time{}, ErrSelection
	}
	zone, err := scheduleZone(value.TimeZone)
	if err != nil {
		return nil, 0, time.Time{}, ErrSelection
	}
	explanations := map[string]PriceExplanation{}
	relevant := []Window{}
	for i := range value.Items {
		item := &value.Items[i]
		for _, line := range lines {
			if line.ItemID != item.ID {
				continue
			}
			relevant = append(relevant, item.AvailabilityWindows...)
			if len(item.AvailabilityWindows) > 0 && !active(item.AvailabilityWindows, now, zone) {
				return nil, 0, time.Time{}, ErrSelection
			}
			for j := range item.Variants {
				variant := &item.Variants[j]
				if variant.ID != line.VariantID {
					continue
				}
				relevant = append(relevant, variant.AvailabilityWindows...)
				if len(variant.AvailabilityWindows) > 0 && !active(variant.AvailabilityWindows, now, zone) {
					return nil, 0, time.Time{}, ErrSelection
				}
				key := item.ID + "/" + variant.ID
				if _, exists := explanations[key]; !exists {
					e := PriceExplanation{Base: variant.Price, EffectiveBase: variant.Price, EvaluatedAt: now}
					var chosen *scheduledOffer
					for k := range value.Offers {
						offer := &value.Offers[k]
						if offer.ItemID != item.ID || offer.VariantID != variant.ID {
							continue
						}
						relevant = append(relevant, offer.Windows...)
						if active(offer.Windows, now, zone) && (chosen == nil || offer.Priority > chosen.Priority || offer.Priority == chosen.Priority && offer.ID < chosen.ID) {
							chosen = offer
						}
					}
					if chosen != nil {
						variant.Price = chosen.Price
						e.OfferID = chosen.ID
						e.EffectiveBase = chosen.Price
					}
					explanations[key] = e
				}
			}
		}
	}
	// Variant-specific extras are materialized per line, preserving one shared
	// selection validator and money-bound checks without mutating retained input.
	quoted := []QuotedLine{}
	var total int64
	for _, line := range lines {
		// Clone only the chosen item, not the complete menu for every line.
		single := menu{Currency: value.Currency, AvailabilityMode: value.AvailabilityMode}
		for _, item := range value.Items {
			if item.ID == line.ItemID {
				single.Items = append(single.Items, item)
				break
			}
		}
		encoded, _ := json.Marshal(single)
		var perLine menu
		_ = json.Unmarshal(encoded, &perLine)
		for i := range perLine.Items {
			item := &perLine.Items[i]
			if item.ID != line.ItemID {
				continue
			}
			for j := range item.Groups {
				for k := range item.Groups[j].Options {
					option := &item.Groups[j].Options[k]
					if price, ok := option.VariantPrices[line.VariantID]; ok {
						option.Price = price
					}
				}
			}
		}
		encoded, _ = json.Marshal(v1Menu(perLine))
		q, amount, err := PriceLocalized(encoded, []SelectionLine{line}, locale)
		if err != nil || amount > MaxTotal-total {
			return nil, 0, time.Time{}, ErrSelection
		}
		total += amount
		e := explanations[line.ItemID+"/"+line.VariantID]
		e.Extras = q[0].UnitPrice - e.EffectiveBase
		q[0].PriceExplanation = &e
		for choiceIndex := range q[0].SelectedOptions {
			choice := &q[0].SelectedOptions[choiceIndex]
			for _, group := range perLine.Items[0].Groups {
				if group.ID == choice.GroupID {
					for _, option := range group.Options {
						if option.ID == choice.OptionID {
							price := option.Price
							choice.Price = &price
						}
					}
				}
			}
		}
		quoted = append(quoted, q[0])
	}
	// Check UTC minute edges rather than adding local wall-clock durations: this
	// naturally covers DST gaps/repetitions. Explicit UTC bounds may be subminute.
	candidates := []time.Time{}
	for at := now.Truncate(time.Minute).Add(time.Minute); at.Before(deadline); at = at.Add(time.Minute) {
		candidates = append(candidates, at)
	}
	for _, w := range relevant {
		for _, bound := range []*time.Time{w.ValidFrom, w.ValidUntil} {
			if bound != nil && bound.After(now) && bound.Before(deadline) {
				candidates = append(candidates, *bound)
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Before(candidates[j]) })
	for _, at := range candidates {
		for _, w := range relevant {
			if active([]Window{w}, at.Add(-time.Nanosecond), zone) != active([]Window{w}, at, zone) {
				return quoted, total, at, nil
			}
		}
	}
	return quoted, total, deadline, nil
}
