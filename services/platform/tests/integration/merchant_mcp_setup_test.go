package integration

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ocp.local/platform/cmd/api/transport/httpapi"
	"ocp.local/platform/internal/core/persistence"
)

// These helpers inspect actual HTTP tool envelopes, not just successful HTTP status.
func toolOutput(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var out struct {
		IsError    bool           `json:"isError"`
		Structured map[string]any `json:"structuredContent"`
	}
	must(t, json.Unmarshal(raw, &out))
	if out.IsError || out.Structured == nil {
		t.Fatalf("tool failed: %s", raw)
	}
	return out.Structured
}
func toolCode(t *testing.T, raw json.RawMessage, code string) {
	t.Helper()
	var out struct {
		IsError bool `json:"isError"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	must(t, json.Unmarshal(raw, &out))
	if !out.IsError || len(out.Content) == 0 || !strings.Contains(out.Content[0].Text, `"code":"`+code+`"`) {
		t.Fatalf("want %s: %s", code, raw)
	}
}

func TestMerchantMCPMenuSetup(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	handler, err := httpapi.NewHandler(f.app, httpapi.WithLocalMerchantMCP())
	must(t, err)
	server := httptest.NewServer(handler)
	defer server.Close()
	_, token, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, f.principal.MerchantID, []string{"offering:read", "offering:write"}, time.Now().Add(time.Hour))
	must(t, err)
	var store persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", f.merchantPath, f.token, "setup-mcp-store-001", `{"display_name":"Setup test"}`, 201), &store))
	call := func(name string, args map[string]any) json.RawMessage {
		return mcpRequest(t, server, token, "tools/call", map[string]any{"name": name, "arguments": args}, 200).Result
	}
	if toolOutput(t, call("inspect_menu_structure", map[string]any{"store": store.Handle}))["revision"] != float64(0) {
		t.Fatal("nonempty new store")
	}
	fields := menu(t)["items"].([]any)[0].(map[string]any)
	delete(fields, "id")
	fields["translations"] = map[string]string{"en": "Kebab", "ar": "كباب"}
	args := map[string]any{"store": store.Handle, "item": "doener", "create": true, "expected_revision": 0, "request_key": "setup-mcp-create-001", "fields": fields}
	if toolOutput(t, call("save_menu_item", args))["draft_revision"] != float64(1) {
		t.Fatal("first creation")
	}
	if toolOutput(t, call("save_menu_item", args))["replayed"] != true {
		t.Fatal("first creation retry")
	}
	args["create"], args["expected_revision"], args["request_key"], args["fields"] = false, 1, "setup-mcp-rename-001", map[string]any{"name": "Renamed kebab"}
	toolOutput(t, call("save_menu_item", args))
	item := toolOutput(t, call("inspect_menu_item", map[string]any{"store": store.Handle, "item": "doener"}))["item"].(map[string]any)
	if item["name"] != "Renamed kebab" || item["translations"].(map[string]any)["en"] != "Kebab" || len(item["variants"].([]any)) != 1 || len(item["option_groups"].([]any)) != 1 {
		t.Fatal(item)
	}
	category := map[string]any{"store": store.Handle, "expected_revision": 2, "request_key": "setup-mcp-cats-0001", "categories": []any{map[string]any{"id": "mains", "name": "Mains", "translations": map[string]string{"de": "Hauptgerichte"}}}, "assignments": map[string]string{"doener": "mains"}}
	toolOutput(t, call("organize_menu_categories", category))
	structure := toolOutput(t, call("inspect_menu_structure", map[string]any{"store": store.Handle}))
	if structure["item_counts"].(map[string]any)["mains"] != float64(1) {
		t.Fatal(structure)
	}
	category["expected_revision"], category["request_key"], category["categories"] = 3, "setup-mcp-badcat-01", []any{}
	delete(category, "assignments")
	toolCode(t, call("organize_menu_categories", category), "invalid_offering")
	args["expected_revision"], args["request_key"], args["fields"] = 3, "setup-mcp-clear-001", map[string]any{"translations": map[string]string{}, "category_id": ""}
	toolOutput(t, call("save_menu_item", args))
	toolCode(t, call("save_menu_item", map[string]any{"store": store.Handle, "item": "doener", "expected_revision": 3, "request_key": "setup-mcp-stale-001", "fields": map[string]any{"name": "Stale"}}), "revision_conflict")
	args["expected_revision"], args["request_key"], args["fields"] = 4, "setup-mcp-invalid-1", map[string]any{"variants": []any{}}
	toolCode(t, call("save_menu_item", args), "invalid_offering")
	args["fields"] = map[string]any{"translations": map[string]string{"fr": "No unsupported locale"}}
	toolCode(t, call("save_menu_item", args), "invalid_offering")
	args["fields"] = map[string]any{"variants": nil}
	raw := call("save_menu_item", args)
	var result struct {
		IsError bool `json:"isError"`
	}
	must(t, json.Unmarshal(raw, &result))
	if !result.IsError {
		t.Fatal("null patch accepted")
	}
	publication, err := persistence.GetPublication(ctx, f.app, f.principal.MerchantID, store.ID)
	must(t, err)
	if publication.Generation != 0 || publication.Revision != nil {
		t.Fatal("setup published")
	}
}
