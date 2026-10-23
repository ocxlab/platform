package integration

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"ocp.local/platform/cmd/api/transport/httpapi"
	"ocp.local/platform/internal/core/persistence"
)

func TestMerchantMCPPublication(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	handler, err := httpapi.NewHandler(f.app, httpapi.WithLocalMerchantMCP())
	must(t, err)
	server := httptest.NewServer(handler)
	defer server.Close()
	_, token, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, f.principal.MerchantID, []string{"offering:read", "offering:write"}, time.Now().Add(time.Hour))
	must(t, err)
	var store persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", f.merchantPath, f.token, "publish-mcp-store-1", `{"display_name":"Publish test"}`, 201), &store))
	f.request(t, "PUT", f.merchantPath+"/"+store.ID.String()+"/offering", f.token, "publish-mcp-menu-01", offeringBody(t, 0, menu(t), "ocp.food.menu"), 201)
	call := func(name string, args map[string]any) json.RawMessage {
		return mcpRequest(t, server, token, "tools/call", map[string]any{"name": name, "arguments": args}, 200).Result
	}
	preview := toolOutput(t, call("preview_menu_publication", map[string]any{"store": store.Handle, "revision": 1}))
	args := map[string]any{"store": store.Handle, "revision": 1, "expected_generation": 0, "review_token": preview["review_token"], "confirmed": false, "request_key": "publish-mcp-first-1"}
	toolCode(t, call("publish_menu_revision", args), "invalid_request")
	args["confirmed"], args["review_token"] = true, "wrong"
	toolCode(t, call("publish_menu_revision", args), "invalid_request")
	args["review_token"] = preview["review_token"]
	published := toolOutput(t, call("publish_menu_revision", args))
	if published["generation"] != float64(1) || published["published_revision"] != float64(1) {
		t.Fatal(published)
	}
	if toolOutput(t, call("publish_menu_revision", args))["replayed"] != true {
		t.Fatal("publication retry")
	}
	args["request_key"] = "publish-mcp-stale-1"
	toolCode(t, call("publish_menu_revision", args), "publication_conflict")
	toolCode(t, call("preview_menu_publication", map[string]any{"store": store.Handle, "revision": 1, "offset": 1, "expected_generation": 0}), "publication_conflict")
	// A new draft does not silently replace the explicitly reviewed revision.
	document := menu(t)
	document["items"].([]any)[0].(map[string]any)["name"] = "New unreviewed name"
	f.request(t, "PUT", f.merchantPath+"/"+store.ID.String()+"/offering", f.token, "publish-mcp-menu-02", offeringBody(t, 1, document, "ocp.food.menu"), 201)
	preview = toolOutput(t, call("preview_menu_publication", map[string]any{"store": store.Handle, "revision": 1}))
	if preview["total_changes"] != float64(0) {
		t.Fatal(preview)
	}
	args["expected_generation"], args["review_token"], args["request_key"] = 1, preview["review_token"], "publish-mcp-exact-1"
	exact := toolOutput(t, call("publish_menu_revision", args))
	if exact["published_revision"] != float64(1) {
		t.Fatal(exact)
	}
	args["revision"], args["expected_generation"], args["request_key"] = 2, 2, "publish-mcp-token-1"
	toolCode(t, call("publish_menu_revision", args), "invalid_request")
	// Read-only grants can preview but cannot publish.
	_, readToken, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, f.principal.MerchantID, []string{"offering:read"}, time.Now().Add(time.Hour))
	must(t, err)
	mcpRequest(t, server, readToken, "tools/call", map[string]any{"name": "publish_menu_revision", "arguments": args}, 400)
}
