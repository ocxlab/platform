package agenttools

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ocp.local/platform/internal/commerce"
	"ocp.local/platform/internal/offering"
	"ocp.local/platform/internal/offering/food"
	"ocp.local/platform/internal/persistence"
)

type itemInput struct {
	Store    string `json:"store" jsonschema:"Managed store handle from find_my_stores"`
	Item     string `json:"item" jsonschema:"Exact item id from inspect_menu; never choose among ambiguous names"`
	Revision int64  `json:"revision,omitempty" jsonschema:"Draft revision returned by inspect_menu; omit for latest"`
}

type extrasInput struct {
	Store    string   `json:"store" jsonschema:"Managed store handle"`
	Item     string   `json:"item" jsonschema:"Exact menu item id"`
	Revision int64    `json:"revision" jsonschema:"Positive draft revision from inspect_menu_item"`
	Groups   []string `json:"groups" jsonschema:"One to three exact extras group ids from inspect_menu_item, in desired order"`
}

func extrasInputSchema() *jsonschema.Schema {
	// Keep field definitions/descriptions derived from the same typed input.
	// A nil Go slice is not an accepted command: advertise a required array,
	// not the SDK's nullable-array default, for cross-client portability.
	schema, err := jsonschema.For[extrasInput](nil)
	if err != nil {
		panic("infer extras tool input schema: " + err.Error())
	}
	schema.Properties["groups"].Type = "array"
	schema.Properties["groups"].Types = nil
	return schema
}

// Read projections retain domain field names and integer minor-unit prices.
// The menu contract bounds variants to 30, groups to 20, and options to 30/group.
type menuPrice struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int64  `json:"price_minor"`
}

type menuGroup struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Min     int         `json:"min_selected"`
	Max     int         `json:"max_selected"`
	Options []menuPrice `json:"options"`
}

type menuItem struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Available bool        `json:"available"`
	Variants  []menuPrice `json:"variants"`
	Groups    []menuGroup `json:"option_groups"`
}

func filterMenuItems(items []json.RawMessage, query string) ([]json.RawMessage, error) {
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) > 100 {
		return nil, commerce.ErrInvalid
	}
	if query == "" {
		return items, nil
	}
	query = strings.ToLower(query)
	matches := make([]json.RawMessage, 0)
	for _, raw := range items {
		var item struct{ ID, Name string }
		if json.Unmarshal(raw, &item) != nil {
			return nil, offering.ErrInvalid
		}
		if strings.Contains(strings.ToLower(item.ID), query) || strings.Contains(strings.ToLower(item.Name), query) {
			matches = append(matches, raw)
		}
	}
	return matches, nil
}

func (s Merchant) loadMenuItem(ctx context.Context, p persistence.Principal, in itemInput) (menuItem, string, int64, error) {
	if !p.AuthorizesMerchant(p.MerchantID, "offering:read") {
		return menuItem{}, "", 0, commerce.ErrForbidden
	}
	if in.Revision < 0 || in.Item == "" || len(in.Item) > 64 {
		return menuItem{}, "", 0, commerce.ErrInvalid
	}
	store, err := s.resolve(ctx, p, in.Store)
	if err != nil {
		return menuItem{}, "", 0, err
	}
	r, err := persistence.GetOffering(ctx, s.DB, p.MerchantID, store, in.Revision)
	if err != nil {
		return menuItem{}, "", 0, err
	}
	if r.Model != food.Model {
		return menuItem{}, "", 0, offering.ErrUnsupported
	}
	var doc struct {
		Currency string     `json:"currency"`
		Items    []menuItem `json:"items"`
	}
	if json.Unmarshal(r.Document, &doc) != nil {
		return menuItem{}, "", 0, offering.ErrInvalid
	}
	for _, item := range doc.Items {
		if item.ID == in.Item {
			return item, doc.Currency, r.Revision, nil
		}
	}
	return menuItem{}, "", 0, persistence.ErrNotFound
}

func (s Merchant) addMenuDetailTools(server *mcp.Server, p persistence.Principal) {
	if !p.AuthorizesMerchant(p.MerchantID, "offering:read") {
		return
	}
	closed := false
	mcp.AddTool(server, &mcp.Tool{
		Name: "inspect_menu_item", Description: "Inspect one draft dish's sizes and full prices, availability, and extras group names/selection limits/counts. No option lists. Use inspect_item_extras only for groups relevant to the task. Does not change a menu.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in itemInput) (*mcp.CallToolResult, map[string]any, error) {
		item, currency, revision, err := s.loadMenuItem(ctx, p, in)
		if err != nil {
			return toolError(err)
		}
		type groupSummary struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Min   int    `json:"min_selected"`
			Max   int    `json:"max_selected"`
			Count int    `json:"option_count"`
		}
		groups := make([]groupSummary, 0, len(item.Groups))
		for _, group := range item.Groups {
			groups = append(groups, groupSummary{group.ID, group.Name, group.Min, group.Max, len(group.Options)})
		}
		return nil, map[string]any{"store": in.Store, "revision": revision, "currency": currency, "state": "private_draft", "item": map[string]any{"id": item.ID, "name": item.Name, "available": item.Available, "variants": item.Variants, "option_groups": groups}}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "inspect_item_extras", Description: "Read options and surcharges (including free extras) for one to three selected groups of one draft dish. Includes required/maximum choice counts. Batch related groups to avoid needless round trips. Use exact ids and the inspected revision; does not change a menu.",
		InputSchema: extrasInputSchema(),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in extrasInput) (*mcp.CallToolResult, map[string]any, error) {
		if in.Revision < 1 || len(in.Groups) < 1 || len(in.Groups) > 3 {
			return toolError(commerce.ErrInvalid)
		}
		seen := make(map[string]bool)
		for _, id := range in.Groups {
			if id == "" || len(id) > 64 || seen[id] {
				return toolError(commerce.ErrInvalid)
			}
			seen[id] = true
		}
		item, currency, revision, err := s.loadMenuItem(ctx, p, itemInput{in.Store, in.Item, in.Revision})
		if err != nil {
			return toolError(err)
		}
		groups := make([]menuGroup, 0, len(in.Groups))
		for _, id := range in.Groups {
			found := false
			for _, group := range item.Groups {
				if group.ID == id {
					groups = append(groups, group)
					found = true
					break
				}
			}
			if !found {
				return toolError(persistence.ErrNotFound)
			}
		}
		return nil, map[string]any{"store": in.Store, "revision": revision, "currency": currency, "state": "private_draft", "item": item.ID, "option_groups": groups}, nil
	})
}
