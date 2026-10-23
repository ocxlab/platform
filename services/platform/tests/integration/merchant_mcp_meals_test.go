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

func TestMerchantMCPMealOffers(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	handler, err := httpapi.NewHandler(f.app, httpapi.WithLocalMerchantMCP())
	must(t, err)
	server := httptest.NewServer(handler)
	defer server.Close()
	_, token, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, f.principal.MerchantID, []string{"offering:read", "offering:write"}, time.Now().Add(time.Hour))
	must(t, err)
	var store persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", f.merchantPath, f.token, "meal-mcp-store-0001", `{"display_name":"Meal test"}`, 201), &store))
	document := menu(t)
	items := document["items"].([]any)
	drink := map[string]any{"id": "drink", "name": "Drink", "available": false, "variants": []any{map[string]any{"id": "normal", "name": "Normal", "price_minor": 250}}, "option_groups": []any{}}
	document["items"] = append(items, drink)
	f.request(t, "PUT", f.merchantPath+"/"+store.ID.String()+"/offering", f.token, "meal-mcp-menu-00001", offeringBody(t, 0, document, "ocp.food.menu"), 201)
	call := func(name string, args map[string]any) json.RawMessage {
		return mcpRequest(t, server, token, "tools/call", map[string]any{"name": name, "arguments": args}, 200).Result
	}
	decode := func(raw json.RawMessage) map[string]any {
		t.Helper()
		var out struct {
			IsError    bool           `json:"isError"`
			Structured map[string]any `json:"structuredContent"`
		}
		must(t, json.Unmarshal(raw, &out))
		if out.IsError || out.Structured == nil {
			t.Fatalf("failed tool: %s", raw)
		}
		return out.Structured
	}
	assertCode := func(raw json.RawMessage, want string) {
		t.Helper()
		var out struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		must(t, json.Unmarshal(raw, &out))
		if !out.IsError || len(out.Content) == 0 || !strings.Contains(out.Content[0].Text, `"code":"`+want+`"`) {
			t.Fatalf("want %s: %s", want, raw)
		}
	}
	if len(decode(call("inspect_meal_offers", map[string]any{"store": store.Handle}))["offers"].([]any)) != 0 {
		t.Fatal("unexpected seed offers")
	}
	offer := map[string]any{"id": "lunch", "name": "Lunch", "translations": map[string]any{"de": "Mittagsmenü"}, "main_item_ids": []string{"doener"}, "surcharge_minor": 100, "addon_choices": []any{map[string]any{"item_id": "drink", "variant_id": "normal"}}}
	args := map[string]any{"store": store.Handle, "expected_revision": 1, "request_key": "meal-mcp-create-001", "create": true, "offer": offer}
	created := decode(call("save_meal_offer", args))
	if created["draft_revision"] != float64(2) || created["replayed"] != false {
		t.Fatal(created)
	}
	summary := decode(call("inspect_meal_offers", map[string]any{"store": store.Handle, "item": "doener", "revision": 2}))
	if summary["matching_offers"] != float64(1) || strings.Contains(string(mustJSON(t, summary)), "addon_choices") {
		t.Fatal(summary)
	}
	detail := decode(call("inspect_meal_offers", map[string]any{"store": store.Handle, "offer": "lunch", "revision": 2}))["offer"].(map[string]any)
	choice := detail["addon_choices"].([]any)[0].(map[string]any)
	if choice["item_name"] != "Drink" || choice["variant_name"] != "Normal" || choice["available"] != false {
		t.Fatal(detail)
	}
	assertCode(call("inspect_meal_offers", map[string]any{"store": store.Handle, "offer": "lunch"}), "invalid_request")
	assertCode(call("inspect_meal_offers", map[string]any{"store": store.Handle, "offer": "missing", "revision": 2}), "not_found")
	assertCode(call("inspect_meal_offers", map[string]any{"store": store.Handle, "item": "missing"}), "not_found")
	if decode(call("save_meal_offer", args))["replayed"] != true {
		t.Fatal("identical create not replayed")
	}
	var original map[string]any
	must(t, json.Unmarshal(mustJSON(t, args), &original))
	args["expected_revision"], args["request_key"] = 2, "meal-mcp-duplicate-1"
	assertCode(call("save_meal_offer", args), "invalid_request")
	args["create"], args["request_key"] = false, "meal-mcp-update-001"
	delete(offer, "translations")
	offer["surcharge_minor"] = 0
	updated := decode(call("save_meal_offer", args))
	if updated["draft_revision"] != float64(3) || updated["replayed"] != false {
		t.Fatal(updated)
	}
	if decode(call("save_meal_offer", original))["draft_revision"] != float64(2) || decode(call("save_meal_offer", original))["replayed"] != true {
		t.Fatal("create replay lost after head advanced")
	}
	// Pinned reads retain old terms even after a concurrent/newer edit.
	retained := decode(call("inspect_meal_offers", map[string]any{"store": store.Handle, "offer": "lunch", "revision": 2}))["offer"].(map[string]any)
	if retained["surcharge_minor"] != float64(100) {
		t.Fatal(retained)
	}
	args["request_key"] = "meal-mcp-stale-0001"
	assertCode(call("save_meal_offer", args), "revision_conflict")
	args["expected_revision"], args["request_key"] = 3, "meal-mcp-update-001"
	assertCode(call("save_meal_offer", args), "idempotency_conflict")
	args["request_key"] = "meal-mcp-invalid-01"
	for _, invalid := range []map[string]any{
		{"surcharge_minor": -1},
		{"surcharge_minor": 1000000001},
		{"main_item_ids": []string{"unknown"}},
		{"addon_choices": []any{map[string]any{"item_id": "drink", "variant_id": "unknown"}}},
		{"addon_choices": []any{map[string]any{"item_id": "doener", "variant_id": "regular"}}}, // Required sauce modifier.
		{"translations": map[string]any{"fr": "Menu"}},
	} {
		var badOffer map[string]any
		must(t, json.Unmarshal(mustJSON(t, offer), &badOffer))
		for k, v := range invalid {
			badOffer[k] = v
		}
		args["offer"] = badOffer
		assertCode(call("save_meal_offer", args), "invalid_offering")
	}
	args["offer"] = offer
	// Read-only grant cannot discover or invoke a writer, including with a valid command.
	_, readToken, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, f.principal.MerchantID, []string{"offering:read"}, time.Now().Add(time.Hour))
	must(t, err)
	listed := mcpRequest(t, server, readToken, "tools/list", map[string]any{}, 200)
	if strings.Contains(string(listed.Result), "save_meal_offer") || !strings.Contains(string(listed.Result), "inspect_meal_offers") {
		t.Fatal(string(listed.Result))
	}
	mcpRequest(t, server, readToken, "tools/call", map[string]any{"name": "save_meal_offer", "arguments": args}, 400)
	other, otherToken, err := persistence.Provision(ctx, f.admin, "Other", []string{"store:write"}, time.Now().Add(time.Hour))
	must(t, err)
	var foreign persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", "/api/v1/merchants/"+other.MerchantID.String()+"/stores", otherToken, "meal-foreign-000001", `{"display_name":"Foreign"}`, 201), &foreign))
	assertCode(call("inspect_meal_offers", map[string]any{"store": foreign.Handle}), "not_found")
	args["store"] = foreign.Handle
	assertCode(call("save_meal_offer", args), "not_found")
	latest, err := persistence.GetOffering(ctx, f.app, f.principal.MerchantID, store.ID, 0)
	must(t, err)
	var saved map[string]any
	must(t, json.Unmarshal(latest.Document, &saved))
	if latest.Revision != 3 || string(mustJSON(t, saved["items"])) != string(mustJSON(t, document["items"])) {
		t.Fatal("unexpected write or changed menu items")
	}
	savedOffer := saved["meal_deals"].([]any)[0].(map[string]any)
	if savedOffer["surcharge_minor"] != float64(0) || savedOffer["translations"].(map[string]any)["de"] != "Mittagsmenü" {
		t.Fatal(savedOffer)
	}
	publication, err := persistence.GetPublication(ctx, f.app, f.principal.MerchantID, store.ID)
	must(t, err)
	if publication.Revision != nil || publication.Generation != 0 {
		t.Fatal("MCP published a meal offer")
	}
	review := decode(call("review_menu_changes", map[string]any{"store": store.Handle, "from_revision": 2, "to_revision": 3}))
	changes := review["changes"].([]any)
	if len(changes) != 1 || changes[0].(map[string]any)["path"] != "/meal_deals/0/surcharge_minor" {
		t.Fatal(review)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	must(t, err)
	return raw
}
