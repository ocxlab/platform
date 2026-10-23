package agenttools

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ocp.local/platform/internal/commerce"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/offering"
	"ocp.local/platform/internal/offering/food"
)

type itemPriceChange struct {
	Variant string `json:"variant,omitempty" jsonschema:"Exact variant id for a full size price; omit when changing an extra"`
	Group   string `json:"group,omitempty" jsonschema:"Exact extras group id; use together with option and omit variant"`
	Option  string `json:"option,omitempty" jsonschema:"Exact extra option id within group"`
	Price   int64  `json:"price_minor" jsonschema:"New non-negative price in minor currency units, including zero for free extras"`
}

type pricesInput struct {
	Store            string            `json:"store" jsonschema:"Managed store handle"`
	Item             string            `json:"item" jsonschema:"Exact dish id; inspect its sizes/extras before editing"`
	ExpectedRevision int64             `json:"expected_revision" jsonschema:"Positive inspected draft revision"`
	Changes          []itemPriceChange `json:"changes" jsonschema:"One to twenty distinct size prices or extra surcharges for this dish, applied atomically"`
	RequestKey       string            `json:"request_key" jsonschema:"Unique 16-128 character retry key; reuse the exact command on retry"`
}

type reviewInput struct {
	Store        string `json:"store" jsonschema:"Managed store handle"`
	FromRevision int64  `json:"from_revision" jsonschema:"Positive immutable draft revision used as the comparison baseline"`
	ToRevision   int64  `json:"to_revision" jsonschema:"Positive immutable draft revision to review, at or after the baseline"`
	Offset       int    `json:"offset,omitempty" jsonschema:"Returned next_offset; keep both revisions unchanged when paging"`
}

func priceInputSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[pricesInput](nil)
	if err != nil {
		panic("infer prices tool input schema: " + err.Error())
	}
	schema.Properties["changes"].Type = "array"
	schema.Properties["changes"].Types = nil
	return schema
}

func (s Merchant) addMenuEditTools(server *mcp.Server, p persistence.Principal) {
	if !p.AuthorizesMerchant(p.MerchantID, "offering:read") {
		return
	}
	closed := false
	mcp.AddTool(server, &mcp.Tool{
		Name: "review_menu_changes", Description: "Review exact field changes between two retained menu revisions, up to 20 per page. Returns before/after values and JSON Pointer locations, not full menus. Explicit revisions keep pages stable. Read-only; review is not approval or publication.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in reviewInput) (*mcp.CallToolResult, map[string]any, error) {
		if !p.AuthorizesMerchant(p.MerchantID, "offering:read") {
			return toolError(commerce.ErrForbidden)
		}
		if in.FromRevision < 1 || in.ToRevision < in.FromRevision || in.Offset < 0 {
			return toolError(commerce.ErrInvalid)
		}
		store, err := s.resolve(ctx, p, in.Store)
		if err != nil {
			return toolError(err)
		}
		from, err := persistence.GetOffering(ctx, s.DB, p.MerchantID, store, in.FromRevision)
		if err != nil {
			return toolError(err)
		}
		to, err := persistence.GetOffering(ctx, s.DB, p.MerchantID, store, in.ToRevision)
		if err != nil {
			return toolError(err)
		}
		if from.Model != food.Model || to.Model != food.Model {
			return toolError(offering.ErrUnsupported)
		}
		changes, err := menuChanges(from.Document, to.Document)
		if err != nil {
			return toolError(err)
		}
		if in.Offset > len(changes) {
			return toolError(commerce.ErrInvalid)
		}
		end := min(in.Offset+20, len(changes))
		out := map[string]any{"store": in.Store, "from_revision": in.FromRevision, "to_revision": in.ToRevision, "changes": changes[in.Offset:end], "total_changes": len(changes), "state": "private_draft", "next_action": "Review all change pages. Publication requires a separate explicit merchant action in the workspace."}
		if end < len(changes) {
			out["next_offset"] = end
		}
		return nil, out, nil
	})
	if !p.AuthorizesMerchant(p.MerchantID, "offering:write") {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: "update_item_prices", Description: "Save one dish's size prices and/or extra surcharges in a private draft. Batch up to 20 distinct targets, using integer minor units and exact inspected ids. All changes succeed together or none do; other menu fields are preserved. Never publishes.",
		InputSchema: priceInputSchema(),
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true, DestructiveHint: &closed, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in pricesInput) (*mcp.CallToolResult, map[string]any, error) {
		if !p.AuthorizesMerchant(p.MerchantID, "offering:read", "offering:write") {
			return toolError(commerce.ErrForbidden)
		}
		if err := validatePriceChanges(in); err != nil {
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
		document, err := editItemPrices(base.Document, in)
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
		return nil, map[string]any{"store": in.Store, "item": in.Item, "draft_revision": revision.Revision, "base_revision": in.ExpectedRevision, "state": "private_draft", "replayed": result.Replayed, "next_action": "Use review_menu_changes with the returned base_revision and draft_revision; publishing remains a separate merchant action."}, nil
	})
}

func validatePriceChanges(in pricesInput) error {
	if in.ExpectedRevision < 1 || in.Item == "" || len(in.Item) > 64 || len(in.Changes) < 1 || len(in.Changes) > 20 {
		return commerce.ErrInvalid
	}
	seen := map[string]bool{}
	for _, change := range in.Changes {
		if change.Price < 0 || change.Price > 1_000_000_000 || len(change.Variant) > 64 || len(change.Group) > 64 || len(change.Option) > 64 {
			return commerce.ErrInvalid
		}
		variant := change.Variant != "" && change.Group == "" && change.Option == ""
		extra := change.Variant == "" && change.Group != "" && change.Option != ""
		if !variant && !extra {
			return commerce.ErrInvalid
		}
		key := change.Variant + "\x00" + change.Group + "\x00" + change.Option
		if seen[key] {
			return commerce.ErrInvalid
		}
		seen[key] = true
	}
	return nil
}

// Start from the immutable inspected revision so replay reproduces the same
// canonical write even when the latest draft has advanced. Never use floats.
func editItemPrices(raw json.RawMessage, in pricesInput) (json.RawMessage, error) {
	if err := validatePriceChanges(in); err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil {
		return nil, offering.ErrInvalid
	}
	var items []map[string]json.RawMessage
	if json.Unmarshal(doc["items"], &items) != nil {
		return nil, offering.ErrInvalid
	}
	for _, item := range items {
		if rawString(item["id"]) != in.Item {
			continue
		}
		var variants []map[string]json.RawMessage
		var groups []map[string]json.RawMessage
		if json.Unmarshal(item["variants"], &variants) != nil {
			return nil, offering.ErrInvalid
		}
		if len(item["option_groups"]) > 0 && json.Unmarshal(item["option_groups"], &groups) != nil {
			return nil, offering.ErrInvalid
		}
		for _, change := range in.Changes {
			found := false
			if change.Variant != "" {
				for _, v := range variants {
					if rawString(v["id"]) == change.Variant {
						v["price_minor"], _ = json.Marshal(change.Price)
						found = true
						break
					}
				}
			} else {
				for _, g := range groups {
					if rawString(g["id"]) != change.Group {
						continue
					}
					var options []map[string]json.RawMessage
					if json.Unmarshal(g["options"], &options) != nil {
						return nil, offering.ErrInvalid
					}
					for _, option := range options {
						if rawString(option["id"]) == change.Option {
							option["price_minor"], _ = json.Marshal(change.Price)
							found = true
							break
						}
					}
					g["options"], _ = json.Marshal(options)
					break
				}
			}
			if !found {
				return nil, persistence.ErrNotFound
			}
		}
		item["variants"], _ = json.Marshal(variants)
		if len(item["option_groups"]) > 0 {
			item["option_groups"], _ = json.Marshal(groups)
		}
		doc["items"], _ = json.Marshal(items)
		return json.Marshal(doc)
	}
	return nil, persistence.ErrNotFound
}

func rawString(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

type menuChange struct {
	Path       string          `json:"path"`
	Before     json.RawMessage `json:"before,omitempty"`
	After      json.RawMessage `json:"after,omitempty"`
	BeforeItem *changeItem     `json:"before_item,omitempty"`
	AfterItem  *changeItem     `json:"after_item,omitempty"`
}

type changeItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func changeItems(raw json.RawMessage) []changeItem {
	var doc struct {
		Items []changeItem `json:"items"`
	}
	_ = json.Unmarshal(raw, &doc)
	return doc.Items
}

func itemAtPointer(items []changeItem, path string) *changeItem {
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "items" {
		return nil
	}
	index, err := strconv.Atoi(parts[2])
	if err != nil || index < 0 || index >= len(items) {
		return nil
	}
	return &items[index]
}

// Leaf-level RFC 6901 pointers bound each change's size even for large imported
// menu replacements. Array positions are revision-local, not stable item ids.
func menuChanges(before, after json.RawMessage) ([]menuChange, error) {
	flatten := func(raw json.RawMessage) (map[string]json.RawMessage, error) {
		var doc any
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.UseNumber()
		if decoder.Decode(&doc) != nil {
			return nil, offering.ErrInvalid
		}
		out := map[string]json.RawMessage{}
		var walk func(string, any)
		walk = func(path string, value any) {
			switch v := value.(type) {
			case map[string]any:
				if len(v) > 0 {
					for k, child := range v {
						walk(path+"/"+strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1"), child)
					}
					return
				}
			case []any:
				if len(v) > 0 {
					for i, child := range v {
						walk(path+"/"+strconv.Itoa(i), child)
					}
					return
				}
			}
			out[path], _ = json.Marshal(value)
		}
		walk("", doc)
		return out, nil
	}
	a, err := flatten(before)
	if err != nil {
		return nil, err
	}
	b, err := flatten(after)
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for key := range a {
		keys[key] = true
	}
	for key := range b {
		keys[key] = true
	}
	paths := make([]string, 0, len(keys))
	for key := range keys {
		paths = append(paths, key)
	}
	sort.Strings(paths)
	changes := make([]menuChange, 0)
	beforeItems, afterItems := changeItems(before), changeItems(after)
	for _, path := range paths {
		if string(a[path]) != string(b[path]) {
			changes = append(changes, menuChange{Path: path, Before: a[path], After: b[path], BeforeItem: itemAtPointer(beforeItems, path), AfterItem: itemAtPointer(afterItems, path)})
		}
	}
	return changes, nil
}
