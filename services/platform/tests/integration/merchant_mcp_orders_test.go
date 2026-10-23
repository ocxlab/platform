package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"ocp.local/platform/cmd/api/transport/httpapi"
	"ocp.local/platform/internal/core/persistence"
	foodstore "ocp.local/platform/internal/food/persistence"
)

func TestMerchantMCPOrders(t *testing.T) {
	f, store, private, guest, guestToken, order := placedOrderFixture(t)
	ctx := context.Background()
	handler, err := httpapi.NewHandler(f.app, httpapi.WithLocalMerchantMCP())
	must(t, err)
	server := httptest.NewServer(handler)
	defer server.Close()
	_, token, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, f.principal.MerchantID, []string{"order:read", "order:manage"}, time.Now().Add(time.Hour))
	must(t, err)
	call := func(name string, args map[string]any) json.RawMessage {
		return mcpRequest(t, server, token, "tools/call", map[string]any{"name": name, "arguments": args}, 200).Result
	}
	page := toolOutput(t, call("list_food_orders", map[string]any{"store": store.Handle}))
	summary := page["orders"].([]any)[0].(map[string]any)
	if summary["order"] != order.ID.String() || summary["reference"] != order.Reference || summary["items"] != nil || summary["history"] != nil {
		t.Fatal(page)
	}
	details := toolOutput(t, call("inspect_food_order", map[string]any{"store": store.Handle, "order": order.ID.String(), "history": true}))
	if len(details["items"].([]any)) != 1 || len(details["history"].([]any)) != 1 || details["revision"] != float64(1) {
		t.Fatal(details)
	}
	args := map[string]any{"store": store.Handle, "order": order.ID.String(), "expected_revision": 1, "decision": "accept", "request_key": "mcp-order-accept-001"}
	accepted := toolOutput(t, call("decide_food_order", args))
	if accepted["acceptance"] != "accepted" || accepted["payment"] != "not_started" || accepted["revision"] != float64(2) {
		t.Fatal(accepted)
	}
	if toolOutput(t, call("decide_food_order", args))["replayed"] != true {
		t.Fatal("decision not replayed")
	}
	args["decision"] = "reject"
	toolCode(t, call("decide_food_order", args), "idempotency_conflict")
	args["request_key"] = "mcp-order-stale-001"
	toolCode(t, call("decide_food_order", args), "order_revision_conflict")
	toolCode(t, call("inspect_food_order", map[string]any{"store": store.Handle, "order": order.ID.String(), "expected_revision": 1}), "order_revision_conflict")
	progress := map[string]any{"store": store.Handle, "order": order.ID.String(), "expected_revision": 2, "action": "start_preparation", "request_key": "mcp-order-start-001"}
	toolCode(t, call("advance_pickup_order", progress), "order_transition_conflict")
	guestPath := guest + "/food/orders/" + order.ID.String()
	f.request(t, "POST", guestPath+"/development-payment", guestToken, "mcp-order-pay-0001", `{"expected_revision":2,"action":"start"}`, 201)
	f.request(t, "POST", guestPath+"/development-payment", guestToken, "mcp-order-paid-001", `{"expected_revision":3,"action":"succeed"}`, 201)
	progress["expected_revision"], progress["action"], progress["request_key"] = 4, "mark_ready", "mcp-order-skip-0001"
	toolCode(t, call("advance_pickup_order", progress), "order_transition_conflict")
	for i, action := range []string{"start_preparation", "mark_ready", "record_collection"} {
		progress["expected_revision"], progress["action"], progress["request_key"] = 4+i, action, fmt.Sprintf("mcp-order-progress-%d", i)
		out := toolOutput(t, call("advance_pickup_order", progress))
		if out["revision"] != float64(5+i) {
			t.Fatal(out)
		}
	}
	// Historical decision replay still returns the original result after pickup.
	args["decision"], args["request_key"] = "accept", "mcp-order-accept-001"
	if toolOutput(t, call("decide_food_order", args))["revision"] != float64(2) {
		t.Fatal("historical retry changed")
	}
	current := toolOutput(t, call("inspect_food_order", map[string]any{"store": store.Handle, "order": order.ID.String(), "history": true}))
	if current["fulfilment_progress"] != "collected" || current["revision"] != float64(7) || len(current["history"].([]any)) != 7 {
		t.Fatal(current)
	}
	var rest foodstore.FoodOrder
	must(t, json.Unmarshal(f.request(t, "GET", private+"/food/orders/"+order.ID.String(), f.token, "", "", 200), &rest))
	if rest.Revision != 7 || string(mustJSON(t, rest.Terms)) != string(mustJSON(t, order.Terms)) {
		t.Fatal("MCP changed agreed terms or diverged from REST")
	}
	_, readToken, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, f.principal.MerchantID, []string{"order:read"}, time.Now().Add(time.Hour))
	must(t, err)
	mcpRequest(t, server, readToken, "tools/call", map[string]any{"name": "decide_food_order", "arguments": args}, 400)
	_, catalogToken, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, f.principal.MerchantID, []string{"offering:read", "offering:write"}, time.Now().Add(time.Hour))
	must(t, err)
	mcpRequest(t, server, catalogToken, "tools/call", map[string]any{"name": "list_food_orders", "arguments": map[string]any{"store": store.Handle}}, 400)
	toolCode(t, call("list_food_orders", map[string]any{"store": store.Handle, "after": "bad-cursor"}), "invalid_request")
	other, _, err := persistence.Provision(ctx, f.admin, "Other MCP merchant", []string{"store:write"}, time.Now().Add(time.Hour))
	must(t, err)
	_, foreignToken, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, other.MerchantID, []string{"order:read", "order:manage"}, time.Now().Add(time.Hour))
	must(t, err)
	foreign := mcpRequest(t, server, foreignToken, "tools/call", map[string]any{"name": "inspect_food_order", "arguments": map[string]any{"store": store.Handle, "order": order.ID.String()}}, 200).Result
	toolCode(t, foreign, "not_found")
}
