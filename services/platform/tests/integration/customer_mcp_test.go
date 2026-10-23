package integration

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http/httptest"
	"ocp.local/platform/cmd/api/transport/httpapi"
	core "ocp.local/platform/internal/core/persistence"
	food "ocp.local/platform/internal/food/persistence"
	"testing"
)

func TestCustomerMCP(t *testing.T) {
	f, shop, private, guest, guestToken, _ := orderFixture(t)
	ctx := context.Background()
	handler, err := httpapi.NewHandler(f.app, httpapi.WithLocalCustomerMCP())
	must(t, err)
	server := httptest.NewServer(handler)
	defer server.Close()
	session, err := food.CreateGuestMCPSession(ctx, f.app, shop.MerchantID, shop.ID)
	must(t, err)
	other, err := food.CreateGuestMCPSession(ctx, f.app, shop.MerchantID, shop.ID)
	must(t, err)
	request := func(token, method string, args map[string]any, status int) json.RawMessage {
		return mcpRequestAt(t, server, "/mcp/customer/food", token, method, args, status).Result
	}
	call := func(name string, args map[string]any) json.RawMessage {
		return request(session.Token, "tools/call", map[string]any{"name": name, "arguments": args}, 200)
	}
	// Resource audience is enforced independently of the token prefix.
	request(guestToken, "tools/list", map[string]any{}, 401)
	request(f.token, "tools/list", map[string]any{}, 401)
	f.request(t, "GET", guest+"/food/orders/"+uuid.NewString(), session.Token, "", "", 401)
	digestGuest, err := core.AuthenticateCustomerMCP(ctx, f.app, session.Token)
	must(t, err)
	if !digestGuest.Guest || digestGuest.StoreID != shop.ID {
		t.Fatal("incorrect customer scope")
	}
	list := toolOutput(t, call("find_food_stores", map[string]any{}))
	if len(list["stores"].([]any)) != 1 {
		t.Fatal(list)
	}
	menu := toolOutput(t, call("inspect_store_menu", map[string]any{"store": shop.Handle}))
	if len(menu["items"].([]any)) != 1 {
		t.Fatal(menu)
	}
	dish := toolOutput(t, call("inspect_food_dish", map[string]any{"store": shop.Handle, "generation": 1, "item": "doener", "groups": []string{"sauce"}}))
	if len(dish["option_groups"].([]any)) != 1 {
		t.Fatal(dish)
	}
	toolCode(t, call("inspect_store_menu", map[string]any{"store": shop.Handle, "generation": 99}), "publication_conflict")
	var selectionBody map[string]any
	must(t, json.Unmarshal([]byte(selection(1)), &selectionBody))
	args := map[string]any{"store": shop.Handle, "generation": 1, "lines": selectionBody["lines"], "note": "Door pickup"}
	basket := toolOutput(t, call("price_food_basket", args))
	id := basket["basket_id"].(string)
	if basket["total_minor"] != float64(1400) {
		t.Fatal(basket)
	}
	foreign := request(other.Token, "tools/call", map[string]any{"name": "inspect_food_basket", "arguments": map[string]any{"basket_id": id}}, 200)
	toolCode(t, foreign, "not_found")
	revised := toolOutput(t, call("revise_food_basket", map[string]any{"basket_id": id, "generation": 1, "changes": []map[string]any{{"line_index": 0, "remove": true}}, "add": selectionBody["lines"]}))
	if revised["basket_id"] == id || revised["total_minor"] != float64(1400) {
		t.Fatal(revised)
	}
	commit := map[string]any{"basket_id": id, "confirmed": false, "total_minor": 1400, "currency": "EUR", "request_key": "mcp-customer-order-001"}
	toolCode(t, call("commit_food_order", commit), "invalid_request")
	commit["confirmed"], commit["total_minor"] = true, 1
	toolCode(t, call("commit_food_order", commit), "invalid_request")
	var count int
	must(t, f.admin.QueryRow("SELECT count(*) FROM commerce.food_orders").Scan(&count))
	if count != 0 {
		t.Fatal("wrong total committed an order")
	}
	commit["total_minor"] = 1400
	order := toolOutput(t, call("commit_food_order", commit))
	if order["payment"] != "not_started" || order["revision"] != float64(1) {
		t.Fatal(order)
	}
	// Quote removal and withdrawal cannot break a successful placement retry.
	_, err = f.admin.Exec("DELETE FROM commerce.food_quotes WHERE id=$1", id)
	must(t, err)
	f.request(t, "PUT", private+"/publication", f.token, "mcp-customer-withdraw-001", `{"expected_generation":1,"revision":null}`, 201)
	if toolOutput(t, call("commit_food_order", commit))["replayed"] != true {
		t.Fatal("placement replay lost")
	}
	commit["total_minor"] = 1
	toolCode(t, call("commit_food_order", commit), "invalid_request")
	commit["total_minor"] = 1400
	track := map[string]any{"order": order["order"], "details": true, "history": true}
	tracked := toolOutput(t, call("track_my_food_order", track))
	if len(tracked["items"].([]any)) != 1 || tracked["note"] != "Door pickup" {
		t.Fatal(tracked)
	}
	toolCode(t, request(other.Token, "tools/call", map[string]any{"name": "track_my_food_order", "arguments": track}, 200), "not_found")
	cancel := map[string]any{"order": order["order"], "expected_revision": 1, "confirmed": true, "request_key": "mcp-customer-cancel-001"}
	cancelled := toolOutput(t, call("cancel_my_food_order", cancel))
	if cancelled["resolution"] != "cancelled" {
		t.Fatal(cancelled)
	}
	if toolOutput(t, call("cancel_my_food_order", cancel))["replayed"] != true {
		t.Fatal("cancellation not replayed")
	}
	_, err = f.admin.Exec("UPDATE commerce.guest_grants SET revoked=true WHERE id=$1", digestGuest.ID)
	must(t, err)
	request(session.Token, "tools/list", map[string]any{}, 401)
	// Even an MCP-shaped secret cannot authenticate when its DB audience differs.
	otherPrincipal, err := core.AuthenticateCustomerMCP(ctx, f.app, other.Token)
	must(t, err)
	_, err = f.admin.Exec("UPDATE commerce.guest_grants SET resource='guest-rest' WHERE id=$1", otherPrincipal.ID)
	must(t, err)
	request(other.Token, "tools/list", map[string]any{}, 401)
}
