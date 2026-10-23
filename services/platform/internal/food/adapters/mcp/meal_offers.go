package agenttools

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ocp.local/platform/internal/core/application"
	"ocp.local/platform/internal/core/offering"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/food/offering"
)

type mealChoice struct {
	ItemID    string `json:"item_id"`
	VariantID string `json:"variant_id"`
}

type mealOffer struct {
	ID           string            `json:"id" jsonschema:"Exact meal-offer id; choose a new id only when explicitly creating"`
	Name         string            `json:"name"`
	Translations map[string]string `json:"translations,omitempty" jsonschema:"Optional de/en/ar names; omit to preserve existing translations, empty object clears them"`
	MainItemIDs  []string          `json:"main_item_ids" jsonschema:"Exact eligible main dish ids, one to 200"`
	Surcharge    int64             `json:"surcharge_minor" jsonschema:"Non-negative upgrade surcharge in minor units; replaces the add-on's standalone price, not the main dish price"`
	AddonChoices []mealChoice      `json:"addon_choices" jsonschema:"One to 200 exact add-on item/variant pairs; no required modifiers or nested deals"`
}

type mealMenu struct {
	Currency string      `json:"currency"`
	Items    []menuItem  `json:"items"`
	Offers   []mealOffer `json:"meal_deals"`
}

type mealReadInput struct {
	Store    string `json:"store" jsonschema:"Managed store handle"`
	Revision int64  `json:"revision,omitempty" jsonschema:"Immutable draft revision; required for continuation or inspecting an exact offer"`
	Item     string `json:"item,omitempty" jsonschema:"Optional exact main dish id to filter offer summaries; omit when using offer"`
	Offer    string `json:"offer,omitempty" jsonschema:"Exact offer id for details and paginated add-on choices; omit for offer summaries"`
	Offset   int    `json:"offset,omitempty" jsonschema:"Returned next_offset; keep revision and selectors unchanged when paging"`
}

type mealSaveInput struct {
	Store            string    `json:"store" jsonschema:"Managed store handle"`
	ExpectedRevision int64     `json:"expected_revision" jsonschema:"Positive inspected draft revision"`
	RequestKey       string    `json:"request_key" jsonschema:"Unique 16-128 character retry key; reuse the identical command on retry"`
	Create           bool      `json:"create,omitempty" jsonschema:"Set true only to create a new offer; default updates an existing offer and never silently creates one"`
	Offer            mealOffer `json:"offer" jsonschema:"Complete offer terms, not a partial patch; other offers and menu items are preserved"`
}

func mealSaveSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[mealSaveInput](nil)
	if err != nil {
		panic("infer meal offer input: " + err.Error())
	}
	for _, field := range []string{"main_item_ids", "addon_choices"} {
		array := schema.Properties["offer"].Properties[field]
		array.Type, array.Types = "array", nil
		array.MinItems, array.MaxItems, array.UniqueItems = new(1), new(200), true
	}
	translations := schema.Properties["offer"].Properties["translations"]
	translations.Type, translations.Types = "object", nil
	return schema
}

func (s Merchant) addMealOfferTools(server *mcp.Server, p persistence.Principal) {
	if !p.AuthorizesMerchant(p.MerchantID, "offering:read") {
		return
	}
	closed := false
	mcp.AddTool(server, &mcp.Tool{
		Name: "inspect_meal_offers", Description: "Read private draft meal offers: up to 20 summaries, optionally filtered by exact main dish id. Supply an offer id and positive revision for its main dish ids and up to 20 named add-on choices with availability. Surcharge upgrades each main unit with one selected add-on, replacing its standalone price; no extra customization or nested offers. This is not a live customer quote.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in mealReadInput) (*mcp.CallToolResult, map[string]any, error) {
		if !p.AuthorizesMerchant(p.MerchantID, "offering:read") {
			return toolError(commerce.ErrForbidden)
		}
		if err := validateMealRead(in); err != nil {
			return toolError(err)
		}
		store, err := s.resolve(ctx, p, in.Store)
		if err != nil {
			return toolError(err)
		}
		base, err := persistence.GetOffering(ctx, s.DB, p.MerchantID, store, in.Revision)
		if err != nil {
			return toolError(err)
		}
		if base.Model != food.Model {
			return toolError(offering.ErrUnsupported)
		}
		out, err := mealOfferPage(base.Document, in)
		if err != nil {
			return toolError(err)
		}
		out["store"], out["revision"], out["state"] = in.Store, base.Revision, "private_draft"
		return nil, out, nil
	})
	if !p.AuthorizesMerchant(p.MerchantID, "offering:write") {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: "save_meal_offer", Description: "Create or revise one meal upgrade offer in a private draft, using inspected main dish and add-on size ids. Send complete offer terms; omitted translations stay unchanged. New ids require create=true; updates require an existing id. Shared Food validation checks references and modifier rules. Atomic revision guard and retry key protect concurrent edits. Never publishes or changes item prices.",
		InputSchema: mealSaveSchema(),
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true, DestructiveHint: &closed, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in mealSaveInput) (*mcp.CallToolResult, map[string]any, error) {
		if !p.AuthorizesMerchant(p.MerchantID, "offering:read", "offering:write") {
			return toolError(commerce.ErrForbidden)
		}
		if err := validateMealSave(in); err != nil {
			return toolError(err)
		}
		store, err := s.resolve(ctx, p, in.Store)
		if err != nil {
			return toolError(err)
		}
		base, err := persistence.GetOffering(ctx, s.DB, p.MerchantID, store, in.ExpectedRevision)
		if err != nil {
			return toolError(err)
		}
		if base.Model != food.Model {
			return toolError(offering.ErrUnsupported)
		}
		document, err := editMealOffer(base.Document, in)
		if err != nil {
			return toolError(err)
		}
		result, err := (commerce.Offerings{DB: s.DB, Models: s.Models}).Write(ctx, p, p.MerchantID, store, in.ExpectedRevision, base.Model, document, in.RequestKey)
		if err != nil {
			return toolError(err)
		}
		var revision persistence.Revision
		if err := json.Unmarshal(result.Body, &revision); err != nil {
			return toolError(err)
		}
		return nil, map[string]any{"store": in.Store, "offer": in.Offer.ID, "base_revision": in.ExpectedRevision, "draft_revision": revision.Revision, "state": "private_draft", "replayed": result.Replayed, "next_action": "Preview the exact revision against the publication and obtain explicit merchant approval before publishing."}, nil
	})
}

func validateMealRead(in mealReadInput) error {
	if in.Revision < 0 || in.Offset < 0 || len(in.Item) > 64 || len(in.Offer) > 64 || (in.Item != "" && in.Offer != "") || ((in.Offset > 0 || in.Offer != "") && in.Revision < 1) {
		return commerce.ErrInvalid
	}
	return nil
}

func mealOfferPage(raw json.RawMessage, in mealReadInput) (map[string]any, error) {
	if err := validateMealRead(in); err != nil {
		return nil, err
	}
	var doc mealMenu
	if json.Unmarshal(raw, &doc) != nil {
		return nil, offering.ErrInvalid
	}
	out := map[string]any{"currency": doc.Currency}
	if in.Offer != "" {
		for _, offer := range doc.Offers {
			if offer.ID != in.Offer {
				continue
			}
			if in.Offset > len(offer.AddonChoices) {
				return nil, commerce.ErrInvalid
			}
			end := min(in.Offset+20, len(offer.AddonChoices))
			choices := make([]any, 0, end-in.Offset)
			for _, choice := range offer.AddonChoices[in.Offset:end] {
				found := false
				for _, item := range doc.Items {
					if item.ID != choice.ItemID {
						continue
					}
					for _, variant := range item.Variants {
						if variant.ID != choice.VariantID {
							continue
						}
						choices = append(choices, map[string]any{"item_id": item.ID, "item_name": item.Name, "variant_id": variant.ID, "variant_name": variant.Name, "available": item.Available})
						found = true
						break
					}
					break
				}
				if !found {
					return nil, offering.ErrInvalid
				}
			}
			out["offer"] = map[string]any{"id": offer.ID, "name": offer.Name, "translations": offer.Translations, "main_item_ids": offer.MainItemIDs, "surcharge_minor": offer.Surcharge, "addon_choices": choices, "addon_choice_count": len(offer.AddonChoices)}
			if end < len(offer.AddonChoices) {
				out["next_offset"] = end
			}
			return out, nil
		}
		return nil, persistence.ErrNotFound
	}
	if in.Item != "" && !slices.ContainsFunc(doc.Items, func(item menuItem) bool { return item.ID == in.Item }) {
		return nil, persistence.ErrNotFound
	}
	offers := make([]any, 0)
	for _, offer := range doc.Offers {
		if in.Item != "" && !slices.Contains(offer.MainItemIDs, in.Item) {
			continue
		}
		offers = append(offers, map[string]any{"id": offer.ID, "name": offer.Name, "surcharge_minor": offer.Surcharge, "main_item_count": len(offer.MainItemIDs), "addon_choice_count": len(offer.AddonChoices)})
	}
	if in.Offset > len(offers) {
		return nil, commerce.ErrInvalid
	}
	end := min(in.Offset+20, len(offers))
	out["offers"], out["matching_offers"] = offers[in.Offset:end], len(offers)
	if end < len(offers) {
		out["next_offset"] = end
	}
	return out, nil
}

func validateMealSave(in mealSaveInput) error {
	if in.ExpectedRevision < 1 || len(in.Offer.MainItemIDs) < 1 || len(in.Offer.MainItemIDs) > 200 || len(in.Offer.AddonChoices) < 1 || len(in.Offer.AddonChoices) > 200 {
		return commerce.ErrInvalid
	}
	// The shared model validator owns text, identifiers, prices, uniqueness,
	// locale, and referential rules. Do not fork that policy for agents.
	return nil
}

func editMealOffer(raw json.RawMessage, in mealSaveInput) (json.RawMessage, error) {
	if err := validateMealSave(in); err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil || doc == nil {
		return nil, offering.ErrInvalid
	}
	offers := make([]map[string]json.RawMessage, 0)
	if prior, ok := doc["meal_deals"]; ok && (json.Unmarshal(prior, &offers) != nil || offers == nil) {
		return nil, offering.ErrInvalid
	}
	index := -1
	for i, offer := range offers {
		if rawString(offer["id"]) == in.Offer.ID {
			index = i
			break
		}
	}
	if index < 0 && !in.Create {
		return nil, persistence.ErrNotFound
	}
	if index >= 0 && in.Create {
		return nil, commerce.ErrInvalid
	}
	var updated map[string]json.RawMessage
	encoded, err := json.Marshal(in.Offer)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(encoded, &updated); err != nil {
		return nil, err
	}
	// omitempty cannot distinguish nil from an explicit empty map on marshal.
	if in.Offer.Translations != nil {
		updated["translations"], _ = json.Marshal(in.Offer.Translations)
	}
	if index < 0 {
		offers = append(offers, updated)
	} else {
		for key, value := range updated {
			offers[index][key] = value
		}
	}
	doc["meal_deals"], _ = json.Marshal(offers)
	return json.Marshal(doc)
}
