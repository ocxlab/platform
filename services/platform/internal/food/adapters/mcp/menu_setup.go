package agenttools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	commerce "ocp.local/platform/internal/core/application"
	"ocp.local/platform/internal/core/offering"
	"ocp.local/platform/internal/core/persistence"
	food "ocp.local/platform/internal/food/offering"
)

type menuCategory struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Translations map[string]string `json:"translations,omitempty"`
}
type itemPatch struct {
	Name         *string            `json:"name,omitempty"`
	Available    *bool              `json:"available,omitempty"`
	Category     *string            `json:"category_id,omitempty" jsonschema:"Empty string removes category assignment"`
	Image        *string            `json:"image_id,omitempty" jsonschema:"Authorized completed image id; empty string removes image"`
	Translations *map[string]string `json:"translations,omitempty" jsonschema:"de/en/ar names; empty object clears; omission preserves"`
	Variants     *[]menuPrice       `json:"variants,omitempty" jsonschema:"Complete replacement size list; omit to preserve existing sizes and translations"`
	Groups       *[]menuGroup       `json:"option_groups,omitempty" jsonschema:"Complete replacement modifier groups including options, defaults and overrides; empty array removes groups"`
}
type saveItemInput struct {
	Store            string    `json:"store"`
	Item             string    `json:"item" jsonschema:"Exact dish id; immutable when updating"`
	ExpectedRevision int64     `json:"expected_revision" jsonschema:"Inspected draft revision; zero only for first menu"`
	RequestKey       string    `json:"request_key"`
	Create           bool      `json:"create,omitempty" jsonschema:"Explicit true to add a new dish; false updates an existing dish"`
	Fields           itemPatch `json:"fields" jsonschema:"Only supplied fields change. New dishes require name, available, variants and option_groups"`
}
type categoriesInput struct {
	Store            string            `json:"store"`
	ExpectedRevision int64             `json:"expected_revision"`
	RequestKey       string            `json:"request_key"`
	Categories       []menuCategory    `json:"categories" jsonschema:"Complete ordered category list, at most 50"`
	Assignments      map[string]string `json:"assignments,omitempty" jsonschema:"Exact dish ids mapped to category ids; empty category removes assignment. Other assignments preserved"`
}

func saveItemSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[saveItemInput](nil)
	if err != nil {
		panic(err)
	}
	// Present patch fields cannot be null: omission and explicit clearing have
	// distinct semantics. The shared Food schema validates the resulting document.
	for name, field := range schema.Properties["fields"].Properties {
		kind := "string"
		switch name {
		case "available":
			kind = "boolean"
		case "translations":
			kind = "object"
		case "variants", "option_groups":
			kind = "array"
		}
		field.Type, field.Types = kind, nil
	}
	return schema
}

func (s Merchant) menuBase(ctx context.Context, p persistence.Principal, handle string, revision int64) (persistence.Revision, error) {
	if revision < 0 {
		return persistence.Revision{}, commerce.ErrInvalid
	}
	store, err := s.resolve(ctx, p, handle)
	if err != nil {
		return persistence.Revision{}, err
	}
	if revision == 0 {
		// Never read "latest" here: retries of first creation reconstruct the same
		// immutable empty base even after a draft has been created.
		return persistence.Revision{Model: food.Model, Document: json.RawMessage(`{"currency":"EUR","availability_mode":"manual","items":[]}`)}, nil
	}
	base, err := persistence.GetOffering(ctx, s.DB, p.MerchantID, store, revision)
	if err == nil && base.Model != food.Model {
		err = offering.ErrUnsupported
	}
	return base, err
}

func (s Merchant) saveMenuDocument(ctx context.Context, p persistence.Principal, handle string, revision int64, key string, document json.RawMessage) (*mcp.CallToolResult, map[string]any, error) {
	store, err := s.resolve(ctx, p, handle)
	if err != nil {
		return toolError(err)
	}
	result, err := (commerce.Offerings{DB: s.DB, Models: s.Models}).Write(ctx, p, p.MerchantID, store, revision, food.Model, document, key)
	if err != nil {
		return toolError(err)
	}
	var saved persistence.Revision
	if err = json.Unmarshal(result.Body, &saved); err != nil {
		return toolError(err)
	}
	return nil, map[string]any{"store": handle, "base_revision": revision, "draft_revision": saved.Revision, "state": "private_draft", "replayed": result.Replayed}, nil
}

func (s Merchant) addMenuSetupTools(server *mcp.Server, p persistence.Principal) {
	if !p.AuthorizesMerchant(p.MerchantID, "offering:read") {
		return
	}
	closed := false
	mcp.AddTool(server, &mcp.Tool{Name: "inspect_menu_structure", Description: "Read ordered categories, their translations and item counts, without dishes or extras. Latest empty stores return revision zero. Use this before organizing a menu.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Store    string `json:"store"`
		Revision int64  `json:"revision,omitempty"`
	}) (*mcp.CallToolResult, map[string]any, error) {
		if in.Revision < 0 {
			return toolError(commerce.ErrInvalid)
		}
		store, err := s.resolve(ctx, p, in.Store)
		if err != nil {
			return toolError(err)
		}
		base, err := persistence.GetOffering(ctx, s.DB, p.MerchantID, store, in.Revision)
		if errors.Is(err, persistence.ErrNotFound) && in.Revision == 0 {
			return nil, map[string]any{"store": in.Store, "revision": 0, "categories": []menuCategory{}, "item_counts": map[string]int{}, "uncategorized": 0}, nil
		}
		if err != nil {
			return toolError(err)
		}
		if base.Model != food.Model {
			return toolError(offering.ErrUnsupported)
		}
		var doc struct {
			Categories []menuCategory `json:"categories"`
			Items      []menuItem     `json:"items"`
		}
		if err = json.Unmarshal(base.Document, &doc); err != nil {
			return toolError(err)
		}
		if doc.Categories == nil {
			doc.Categories = []menuCategory{}
		}
		counts := map[string]int{}
		for _, item := range doc.Items {
			counts[item.Category]++
		}
		uncategorized := counts[""]
		delete(counts, "")
		return nil, map[string]any{"store": in.Store, "revision": base.Revision, "categories": doc.Categories, "item_counts": counts, "uncategorized": uncategorized}, nil
	})
	if !p.AuthorizesMerchant(p.MerchantID, "offering:write") {
		return
	}
	mcp.AddTool(server, &mcp.Tool{Name: "save_menu_item", Description: "Add or correct one private draft dish. Supply only changed fields; supplied variants or option_groups replace that entire list, including translations/defaults/overrides. Inspect first; use update_item_prices for price-only changes. New dishes require create=true and all mandatory fields. Shared validation prevents broken meal/category/size references. Never publishes.", InputSchema: saveItemSchema(), Annotations: &mcp.ToolAnnotations{IdempotentHint: true, OpenWorldHint: &closed}}, func(ctx context.Context, _ *mcp.CallToolRequest, in saveItemInput) (*mcp.CallToolResult, map[string]any, error) {
		base, err := s.menuBase(ctx, p, in.Store, in.ExpectedRevision)
		if err != nil {
			return toolError(err)
		}
		document, err := patchMenuItem(base.Document, in)
		if err != nil {
			return toolError(err)
		}
		return s.saveMenuDocument(ctx, p, in.Store, in.ExpectedRevision, in.RequestKey, document)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "organize_menu_categories", Description: "Replace the ordered category list and explicitly assign selected dishes, atomically in a private draft. Omitted dish assignments stay unchanged. Empty assignment removes a category. Removed referenced categories must be reassigned in this same command. Never deletes dishes or publishes.", Annotations: &mcp.ToolAnnotations{IdempotentHint: true, OpenWorldHint: &closed}}, func(ctx context.Context, _ *mcp.CallToolRequest, in categoriesInput) (*mcp.CallToolResult, map[string]any, error) {
		if in.ExpectedRevision < 1 || in.Categories == nil || len(in.Categories) > 50 || len(in.Assignments) > 200 {
			return toolError(commerce.ErrInvalid)
		}
		base, err := s.menuBase(ctx, p, in.Store, in.ExpectedRevision)
		if err != nil {
			return toolError(err)
		}
		document, err := organizeCategories(base.Document, in)
		if err != nil {
			return toolError(err)
		}
		return s.saveMenuDocument(ctx, p, in.Store, in.ExpectedRevision, in.RequestKey, document)
	})
}

func patchMenuItem(raw json.RawMessage, in saveItemInput) (json.RawMessage, error) {
	if in.Item == "" || len(in.Item) > 64 || in.ExpectedRevision < 0 {
		return nil, commerce.ErrInvalid
	}
	fields, err := json.Marshal(in.Fields)
	if err != nil {
		return nil, err
	}
	var patch map[string]json.RawMessage
	if json.Unmarshal(fields, &patch) != nil || len(patch) == 0 {
		return nil, commerce.ErrInvalid
	}
	var doc map[string]json.RawMessage
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil || json.Unmarshal(doc["items"], &items) != nil {
		return nil, offering.ErrInvalid
	}
	index := -1
	for i, item := range items {
		var id string
		_ = json.Unmarshal(item["id"], &id)
		if id == in.Item {
			index = i
			break
		}
	}
	if (index < 0) != in.Create {
		return nil, commerce.ErrInvalid
	}
	if in.Create {
		if in.Fields.Name == nil || in.Fields.Available == nil || in.Fields.Variants == nil || in.Fields.Groups == nil {
			return nil, commerce.ErrInvalid
		}
		item := map[string]json.RawMessage{}
		item["id"], _ = json.Marshal(in.Item)
		items = append(items, item)
		index = len(items) - 1
	}
	for key, value := range patch {
		if (key == "image_id" || key == "category_id") && string(value) == `""` {
			delete(items[index], key)
		} else {
			items[index][key] = value
		}
	}
	doc["items"], _ = json.Marshal(items)
	return json.Marshal(doc)
}

func organizeCategories(raw json.RawMessage, in categoriesInput) (json.RawMessage, error) {
	var doc map[string]json.RawMessage
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil || json.Unmarshal(doc["items"], &items) != nil {
		return nil, offering.ErrInvalid
	}
	found := 0
	for _, item := range items {
		var id string
		_ = json.Unmarshal(item["id"], &id)
		if category, ok := in.Assignments[id]; ok {
			found++
			if category == "" {
				delete(item, "category_id")
			} else {
				item["category_id"], _ = json.Marshal(category)
			}
		}
	}
	if found != len(in.Assignments) {
		return nil, persistence.ErrNotFound
	}
	doc["categories"], _ = json.Marshal(in.Categories)
	doc["items"], _ = json.Marshal(items)
	return json.Marshal(doc)
}
