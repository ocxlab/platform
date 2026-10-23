package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"ocp.local/platform/internal/infra/database"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"ocp.local/platform/cmd/api/transport/httpapi"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/infra/telemetry"
	"ocp.local/platform/tests/devsetup"
)

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func mcpRequest(t *testing.T, server *httptest.Server, token, method string, params any, status int) rpcResponse {
	t.Helper()
	if p, ok := params.(map[string]any); ok {
		p["_meta"] = map[string]any{"io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": map[string]any{}}
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	r, err := http.NewRequest("POST", server.URL+"/mcp/merchant", strings.NewReader(string(body)))
	must(t, err)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	r.Header.Set("Mcp-Method", method)
	if p, ok := params.(map[string]any); ok {
		if name, ok := p["name"].(string); ok {
			r.Header.Set("Mcp-Name", name)
		}
	}
	response, err := server.Client().Do(r)
	must(t, err)
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	must(t, err)
	if response.StatusCode != status {
		t.Fatalf("MCP %s status=%d want=%d: %s", method, response.StatusCode, status, data)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("cacheable MCP response")
	}
	var out rpcResponse
	if status == 200 {
		must(t, json.Unmarshal(data, &out))
		if len(out.Error) > 0 {
			t.Fatalf("RPC error: %s", out.Error)
		}
	}
	return out
}

func TestMerchantMCP(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	exporter := tracetest.NewInMemoryExporter()
	traces := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	reader := sdkmetric.NewManualReader()
	metrics := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer traces.Shutdown(ctx)
	defer metrics.Shutdown(ctx)
	observer, err := telemetry.NewObserver(traces, metrics, nil)
	must(t, err)
	handler, err := httpapi.NewHandler(f.app, httpapi.WithLocalMerchantMCP(), httpapi.WithTelemetry(observer))
	must(t, err)
	server := httptest.NewServer(handler)
	defer server.Close()
	p, token, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, f.principal.MerchantID, []string{"store:read", "offering:read", "offering:write"}, time.Now().Add(time.Hour))
	must(t, err)
	var store persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", f.merchantPath, f.token, "mcp-store-00000001", `{"display_name":"MCP restaurant"}`, 201), &store))
	path := f.merchantPath + "/" + store.ID.String() + "/offering"
	f.request(t, "PUT", path, f.token, "mcp-menu-00000001", offeringBody(t, 0, menu(t), "ocp.food.menu"), 201)
	// Each audience rejects the other's otherwise valid grant.
	f.request(t, "GET", f.merchantPath, token, "", "", 401)
	mcpRequest(t, server, f.token, "tools/list", map[string]any{}, 401)
	_, adminToken, err := persistence.ProvisionProvider(ctx, f.admin, []string{"publication:read"}, time.Now().Add(time.Hour))
	must(t, err)
	mcpRequest(t, server, adminToken, "tools/list", map[string]any{}, 401)
	listed := mcpRequest(t, server, token, "tools/list", map[string]any{}, 200)
	var tools struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	must(t, json.Unmarshal(listed.Result, &tools))
	if len(tools.Tools) != 7 {
		t.Fatalf("tools=%s", listed.Result)
	}
	t.Run("released_sdk_client", func(t *testing.T) {
		client := &http.Client{Transport: mcpBearerTransport{token: token, base: server.Client().Transport}, Timeout: 10 * time.Second}
		session, err := mcp.NewClient(&mcp.Implementation{Name: "ocp-integration-client", Version: "0.1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp/merchant", HTTPClient: client, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
		must(t, err)
		defer session.Close()
		listed, err := session.ListTools(ctx, nil)
		must(t, err)
		if len(listed.Tools) != 7 {
			t.Fatal("SDK tool discovery")
		}
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "find_my_stores", Arguments: map[string]any{}})
		must(t, err)
		if result.IsError {
			t.Fatal("SDK merchant lookup rejected")
		}
	})
	call := func(name string, args map[string]any) json.RawMessage {
		return mcpRequest(t, server, token, "tools/call", map[string]any{"name": name, "arguments": args}, 200).Result
	}
	assertCode := func(raw json.RawMessage, want string) {
		t.Helper()
		var r struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		must(t, json.Unmarshal(raw, &r))
		if !r.IsError || len(r.Content) == 0 || !strings.Contains(r.Content[0].Text, `"code":"`+want+`"`) {
			t.Fatalf("want %s: %s", want, raw)
		}
	}
	inspect := call("inspect_menu", map[string]any{"store": store.Handle})
	if !strings.Contains(string(inspect), `"revision":1`) {
		t.Fatalf("menu=%s", inspect)
	}
	// The larger acceptance catalog stays bounded and pins subsequent pages to
	// an immutable revision rather than silently mixing edits during pagination.
	demo, err := devsetup.EnsureRestaurant(ctx, f.admin)
	must(t, err)
	_, demoToken, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, demo.MerchantID, []string{"offering:read"}, time.Now().Add(time.Hour))
	must(t, err)
	page := mcpRequest(t, server, demoToken, "tools/call", map[string]any{"name": "inspect_menu", "arguments": map[string]any{"store": demo.Handle}}, 200)
	var menuPage struct {
		Structured struct {
			Items    []any `json:"items"`
			Next     int   `json:"next_offset"`
			Total    int   `json:"total_items"`
			Revision int64 `json:"revision"`
		} `json:"structuredContent"`
	}
	must(t, json.Unmarshal(page.Result, &menuPage))
	if len(menuPage.Structured.Items) != 20 || menuPage.Structured.Next != 20 || menuPage.Structured.Total != 48 || len(page.Result) > 20000 {
		t.Fatalf("unbounded or wrong page: %s", page.Result)
	}
	next := mcpRequest(t, server, demoToken, "tools/call", map[string]any{"name": "inspect_menu", "arguments": map[string]any{"store": demo.Handle, "offset": 40, "revision": menuPage.Structured.Revision}}, 200)
	must(t, json.Unmarshal(next.Result, &menuPage))
	if len(menuPage.Structured.Items) != 8 {
		t.Fatal("missing continuation")
	}
	t.Run("progressive_information", func(t *testing.T) {
		demoCall := func(name string, args map[string]any) json.RawMessage {
			return mcpRequest(t, server, demoToken, "tools/call", map[string]any{"name": name, "arguments": args}, 200).Result
		}
		decode := func(raw json.RawMessage) map[string]any {
			t.Helper()
			var result struct {
				IsError    bool           `json:"isError"`
				Structured map[string]any `json:"structuredContent"`
			}
			must(t, json.Unmarshal(raw, &result))
			if result.IsError || result.Structured == nil {
				t.Fatalf("read failed: %s", raw)
			}
			return result.Structured
		}
		// A focused intent needs neither three menu pages nor all dishes' extras.
		searchWire := demoCall("inspect_menu", map[string]any{"store": demo.Handle, "query": " DÖNER IM FLADENBROT "})
		search := decode(searchWire)
		if search["matching_items"] != float64(1) || search["total_items"] != float64(48) || strings.Contains(string(searchWire), `"options"`) {
			t.Fatalf("bad search projection: %s", searchWire)
		}
		itemWire := demoCall("inspect_menu_item", map[string]any{"store": demo.Handle, "item": "doener", "revision": 1})
		item := decode(itemWire)["item"].(map[string]any)
		variants := item["variants"].([]any)
		groups := item["option_groups"].([]any)
		if len(variants) != 2 || variants[1].(map[string]any)["price_minor"] != float64(950) || len(groups) != 2 || strings.Contains(string(itemWire), `"options"`) {
			t.Fatalf("bad item projection: %s", itemWire)
		}
		if groups[0].(map[string]any)["min_selected"] != float64(1) || groups[0].(map[string]any)["option_count"] != float64(4) {
			t.Fatal("missing required choice guidance")
		}
		extrasWire := demoCall("inspect_item_extras", map[string]any{"store": demo.Handle, "item": "doener", "revision": 1, "groups": []string{"extras", "sauce"}})
		extraGroups := decode(extrasWire)["option_groups"].([]any)
		if len(extraGroups) != 2 || extraGroups[0].(map[string]any)["id"] != "extras" {
			t.Fatal("batch/order not preserved")
		}
		options := extraGroups[0].(map[string]any)["options"].([]any)
		if options[0].(map[string]any)["price_minor"] != float64(100) || options[2].(map[string]any)["price_minor"] != float64(0) {
			t.Fatal("lost paid/free extras")
		}
		for _, invalid := range []map[string]any{
			{"store": demo.Handle, "item": "doener", "revision": 0, "groups": []string{"extras"}},
			{"store": demo.Handle, "item": "doener", "revision": 1, "groups": []string{}},
			{"store": demo.Handle, "item": "doener", "revision": 1, "groups": []string{"extras", "extras"}},
			{"store": demo.Handle, "item": "doener", "revision": 1, "groups": []string{"a", "b", "c", "d"}},
		} {
			assertCode(demoCall("inspect_item_extras", invalid), "invalid_request")
		}
		assertCode(demoCall("inspect_menu_item", map[string]any{"store": demo.Handle, "item": "not-an-item"}), "not_found")
		assertCode(demoCall("inspect_item_extras", map[string]any{"store": demo.Handle, "item": "doener", "revision": 1, "groups": []string{"extras", "not-a-group"}}), "not_found")
		empty := decode(demoCall("inspect_menu", map[string]any{"store": demo.Handle, "query": "not-on-this-menu"}))
		if empty["matching_items"] != float64(0) || len(empty["items"].([]any)) != 0 {
			t.Fatal("empty search should return an empty list")
		}
		assertCode(demoCall("inspect_menu", map[string]any{"store": demo.Handle, "query": strings.Repeat("x", 101)}), "invalid_request")
		r, err := persistence.GetOffering(ctx, f.app, demo.MerchantID, demo.ID, 0)
		must(t, err)
		if r.Revision != 1 {
			t.Fatal("read tools mutated the menu")
		}
		// Byte counts include both MCP text and structured copies, not a tokenizer
		// estimate or an assertion about model latency/quality.
		t.Logf("known-store workflow: calls=3 search_result_bytes=%d item_result_bytes=%d batched_extras_result_bytes=%d total_result_bytes=%d full_menu_document_bytes=%d tool_list_result_bytes=%d", len(searchWire), len(itemWire), len(extrasWire), len(searchWire)+len(itemWire)+len(extrasWire), len(r.Document), len(listed.Result))
	})
	var firstItem string
	items := menu(t)["items"].([]any)
	firstItem = items[0].(map[string]any)["id"].(string)
	args := map[string]any{"store": store.Handle, "item": firstItem, "available": false, "expected_revision": 1, "request_key": "mcp-availability-0001"}
	changed := call("set_item_availability", args)
	if !strings.Contains(string(changed), `"draft_revision":2`) || strings.Contains(string(changed), `"isError":true`) {
		t.Fatalf("change=%s", changed)
	}
	replayed := call("set_item_availability", args)
	if !strings.Contains(string(replayed), `"replayed":true`) {
		t.Fatalf("retry=%s", replayed)
	}
	args["available"] = true
	assertCode(call("set_item_availability", args), "idempotency_conflict")
	args["request_key"] = "mcp-stale-base-0001"
	assertCode(call("set_item_availability", args), "revision_conflict")
	draft, err := persistence.GetOffering(ctx, f.app, p.MerchantID, store.ID, 0)
	must(t, err)
	if draft.Revision != 2 {
		t.Fatal("retry created another revision")
	}
	var doc map[string]any
	must(t, json.Unmarshal(draft.Document, &doc))
	expected := menu(t)
	expected["items"].([]any)[0].(map[string]any)["available"] = false
	a, _ := json.Marshal(doc)
	b, _ := json.Marshal(expected)
	if string(a) != string(b) {
		t.Fatal("targeted change lost other menu fields")
	}
	// A retained revision remains a consistent read after another draft is saved.
	oldItem := call("inspect_menu_item", map[string]any{"store": store.Handle, "item": firstItem, "revision": 1})
	latestItem := call("inspect_menu_item", map[string]any{"store": store.Handle, "item": firstItem})
	if !strings.Contains(string(oldItem), `"available":true`) || !strings.Contains(string(latestItem), `"available":false`) {
		t.Fatal("item reads ignored immutable/requested revision")
	}
	// Nothing was published by an agent draft mutation.
	publication, err := persistence.GetPublication(ctx, f.app, p.MerchantID, store.ID)
	must(t, err)
	if publication.Revision != nil {
		t.Fatal("agent published a draft")
	}
	t.Run("focused_prices_and_review", func(t *testing.T) {
		args := map[string]any{"store": store.Handle, "item": firstItem, "expected_revision": 2, "request_key": "mcp-price-update-0001", "changes": []any{
			map[string]any{"variant": "regular", "price_minor": 750},
			map[string]any{"group": "sauce", "option": "garlic", "price_minor": 100},
		}}
		changed := call("update_item_prices", args)
		if !strings.Contains(string(changed), `"draft_revision":3`) || strings.Contains(string(changed), `"isError":true`) {
			t.Fatalf("prices=%s", changed)
		}
		if replayed := call("update_item_prices", args); !strings.Contains(string(replayed), `"replayed":true`) {
			t.Fatalf("retry=%s", replayed)
		}
		args["changes"].([]any)[0].(map[string]any)["price_minor"] = 800
		assertCode(call("update_item_prices", args), "idempotency_conflict")
		args["request_key"] = "mcp-price-stale-0001"
		assertCode(call("update_item_prices", args), "revision_conflict")
		args["expected_revision"] = 3
		args["request_key"] = "mcp-price-partial-0001"
		args["changes"].([]any)[1].(map[string]any)["option"] = "missing"
		assertCode(call("update_item_prices", args), "not_found")
		args["changes"].([]any)[1] = map[string]any{"variant": "regular", "price_minor": 800}
		assertCode(call("update_item_prices", args), "invalid_request")
		args["changes"] = []any{map[string]any{"variant": "regular", "price_minor": -1}}
		assertCode(call("update_item_prices", args), "invalid_request")
		args["changes"] = []any{map[string]any{"variant": "regular", "price_minor": 0.5}}
		// The SDK rejects fractional minor units before invoking application code.
		client := &http.Client{Transport: mcpBearerTransport{token: token, base: server.Client().Transport}, Timeout: 10 * time.Second}
		session, err := mcp.NewClient(&mcp.Implementation{Name: "price-validation"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp/merchant", HTTPClient: client, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
		must(t, err)
		defer session.Close()
		fraction, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "update_item_prices", Arguments: args})
		if err == nil && !fraction.IsError {
			t.Fatal("fractional money accepted")
		}
		r, err := persistence.GetOffering(ctx, f.app, p.MerchantID, store.ID, 0)
		must(t, err)
		if r.Revision != 3 {
			t.Fatal("failed batch created a revision")
		}
		var document map[string]any
		must(t, json.Unmarshal(r.Document, &document))
		want := menu(t)
		dish := want["items"].([]any)[0].(map[string]any)
		dish["available"] = false
		dish["variants"].([]any)[0].(map[string]any)["price_minor"] = float64(750)
		dish["option_groups"].([]any)[0].(map[string]any)["options"].([]any)[0].(map[string]any)["price_minor"] = float64(100)
		a, _ := json.Marshal(document)
		b, _ := json.Marshal(want)
		if string(a) != string(b) {
			t.Fatalf("price edit lost menu fields: %s want=%s", a, b)
		}
		review := call("review_menu_changes", map[string]any{"store": store.Handle, "from_revision": 1, "to_revision": 3})
		var result struct {
			Structured struct {
				Total   int `json:"total_changes"`
				Changes []struct {
					Path   string `json:"path"`
					Before any    `json:"before"`
					After  any    `json:"after"`
				} `json:"changes"`
			} `json:"structuredContent"`
		}
		must(t, json.Unmarshal(review, &result))
		if result.Structured.Total != 3 || len(result.Structured.Changes) != 3 || result.Structured.Changes[0].Path != "/items/0/available" {
			t.Fatalf("review=%s", review)
		}
		assertCode(call("review_menu_changes", map[string]any{"store": store.Handle, "from_revision": 3, "to_revision": 1}), "invalid_request")
		assertCode(call("review_menu_changes", map[string]any{"store": store.Handle, "from_revision": 1, "to_revision": 3, "offset": 4}), "invalid_request")
		assertCode(call("review_menu_changes", map[string]any{"store": store.Handle, "from_revision": 1, "to_revision": 999}), "not_found")
		empty := call("review_menu_changes", map[string]any{"store": store.Handle, "from_revision": 3, "to_revision": 3})
		if !strings.Contains(string(empty), `"changes":[]`) {
			t.Fatalf("empty=%s", empty)
		}
		publication, err := persistence.GetPublication(ctx, f.app, p.MerchantID, store.ID)
		must(t, err)
		if publication.Revision != nil || publication.Generation != 0 {
			t.Fatal("editing/review published menu")
		}
		// REST changes are visible to the same review operation.
		seed, err := persistence.GetOffering(ctx, f.app, demo.MerchantID, demo.ID, 1)
		must(t, err)
		var many map[string]any
		must(t, json.Unmarshal(seed.Document, &many))
		f.request(t, "PUT", path, f.token, "mcp-review-menu-00001", offeringBody(t, 3, many, "ocp.food.menu"), 201)
		for _, raw := range many["items"].([]any) {
			item := raw.(map[string]any)
			item["name"] = item["name"].(string) + " revised"
		}
		f.request(t, "PUT", path, f.token, "mcp-review-menu-00002", offeringBody(t, 4, many, "ocp.food.menu"), 201)
		reviewArgs := map[string]any{"store": store.Handle, "from_revision": 4, "to_revision": 5}
		firstPage := call("review_menu_changes", reviewArgs)
		var page struct {
			Structured struct {
				Changes []json.RawMessage `json:"changes"`
				Next    *int              `json:"next_offset"`
				Total   int               `json:"total_changes"`
			} `json:"structuredContent"`
		}
		must(t, json.Unmarshal(firstPage, &page))
		if len(page.Structured.Changes) != 20 || page.Structured.Next == nil || *page.Structured.Next != 20 || page.Structured.Total != 48 {
			t.Fatalf("unbounded/incomplete review: %s", firstPage)
		}
		// A newer head cannot change the retained comparison or its pages.
		many["items"].([]any)[0].(map[string]any)["available"] = false
		f.request(t, "PUT", path, f.token, "mcp-review-menu-00003", offeringBody(t, 5, many, "ocp.food.menu"), 201)
		for offset, count := range map[int]int{20: 20, 40: 8, 48: 0} {
			reviewArgs["offset"] = offset
			raw := call("review_menu_changes", reviewArgs)
			page.Structured.Next = nil
			must(t, json.Unmarshal(raw, &page))
			if len(page.Structured.Changes) != count || page.Structured.Total != 48 || (offset >= 40 && page.Structured.Next != nil) {
				t.Fatalf("bad continuation: %s", raw)
			}
		}
	})
	other, otherToken, err := persistence.Provision(ctx, f.admin, "Other", []string{"store:write"}, time.Now().Add(time.Hour))
	must(t, err)
	var otherStore persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", "/api/v1/merchants/"+other.MerchantID.String()+"/stores", otherToken, "other-store-000001", `{"display_name":"Other store"}`, 201), &otherStore))
	assertCode(call("inspect_menu", map[string]any{"store": otherStore.Handle}), "not_found")
	assertCode(call("inspect_menu_item", map[string]any{"store": otherStore.Handle, "item": firstItem}), "not_found")
	assertCode(call("inspect_item_extras", map[string]any{"store": otherStore.Handle, "item": firstItem, "revision": 1, "groups": []string{"sauce"}}), "not_found")
	assertCode(call("review_menu_changes", map[string]any{"store": otherStore.Handle, "from_revision": 1, "to_revision": 3}), "not_found")
	assertCode(call("update_item_prices", map[string]any{"store": otherStore.Handle, "item": firstItem, "expected_revision": 1, "request_key": "foreign-price-00001", "changes": []any{map[string]any{"variant": "regular", "price_minor": 1}}}), "not_found")
	// Revocation applies even to a previously successful request key.
	_, err = f.admin.ExecContext(ctx, `UPDATE auth.grants SET revoked=true WHERE id=$1`, p.ID)
	must(t, err)
	mcpRequest(t, server, token, "tools/list", map[string]any{}, 401)
	_, expired, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, p.MerchantID, []string{"store:read"}, time.Now().Add(-time.Minute))
	must(t, err)
	mcpRequest(t, server, expired, "tools/list", map[string]any{}, 401)
	if p.ID == uuid.Nil {
		t.Fatal("missing grant identity")
	}
	// A business conflict inside HTTP 200 is a rejected tool span/metric, and
	// the tool span is correlated with the enclosing HTTP request.
	spans := exporter.GetSpans()
	parents := map[string]bool{}
	nestedRejected := false
	for _, span := range spans {
		if span.Name == "POST /mcp/merchant" {
			parents[span.SpanContext.SpanID().String()] = true
		}
	}
	for _, span := range spans {
		if span.Name != "mcp set_item_availability" {
			continue
		}
		for _, a := range span.Attributes {
			if string(a.Key) == "ocp.outcome" && a.Value.AsString() == "rejected" && parents[span.Parent.SpanID().String()] {
				nestedRejected = true
			}
		}
	}
	if !nestedRejected {
		t.Fatal("tool conflict was not correlated/classified independently of HTTP 200")
	}
	var collected metricdata.ResourceMetrics
	must(t, reader.Collect(ctx, &collected))
	wire, _ := json.Marshal(collected)
	if !strings.Contains(string(wire), "ocp.mcp.tool.calls") || strings.Contains(string(wire), token) || strings.Contains(string(wire), store.ID.String()) {
		t.Fatal("missing or unsafe tool metrics")
	}
}

func TestMerchantResourceGrantUpgrade(t *testing.T) {
	f := setupAtVersion(t, 12)
	ctx := context.Background()
	must(t, database.Migrate(ctx, f.admin))
	// Existing REST credentials keep working after the new resource column.
	p, err := persistence.Authenticate(ctx, f.app, f.token)
	must(t, err)
	if p.ID != f.principal.ID {
		t.Fatal("changed existing grant")
	}
	_, token, err := persistence.ProvisionMerchantMCPGrant(ctx, f.admin, p.MerchantID, []string{"store:read"}, time.Now().Add(time.Hour))
	must(t, err)
	if _, err = persistence.Authenticate(ctx, f.app, token); err != persistence.ErrUnauthenticated {
		t.Fatal("MCP token accepted by REST")
	}
	if _, err = persistence.AuthenticateResource(ctx, f.app, f.token, persistence.MerchantMCP); err != persistence.ErrUnauthenticated {
		t.Fatal("REST token accepted by MCP")
	}
	if _, err = persistence.AuthenticateResource(ctx, f.app, token, "unrecognized"); err != persistence.ErrUnauthenticated {
		t.Fatal("unknown audience accepted")
	}
}

type mcpBearerTransport struct {
	token string
	base  http.RoundTripper
}

func (a mcpBearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	copy.Header.Set("Authorization", "Bearer "+a.token)
	return a.base.RoundTrip(copy)
}
