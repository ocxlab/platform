package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http/httptest"
	"ocp.local/platform/cmd/api/transport/httpapi"
	commerce "ocp.local/platform/internal/core/application"
	core "ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/infra/database"
	"sync"
	"testing"
	"time"
)

func TestAccountRESTAndMCP(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	handler, err := httpapi.NewHandler(f.app, httpapi.WithLocalAccountMCP(), httpapi.WithLocalMerchantMCP())
	must(t, err)
	server := httptest.NewServer(handler)
	defer server.Close()
	for _, kind := range []string{"merchant", "customer"} {
		t.Run(kind, func(t *testing.T) {
			mid := uuid.Nil
			if kind == "merchant" {
				mid = f.principal.MerchantID
			}
			id, err := core.ProvisionAccount(ctx, f.admin, kind, "Example owner", mid)
			must(t, err)
			grant := func(transport string, scopes []string) string {
				token, e := core.ProvisionAccountGrant(ctx, f.admin, id, kind, transport, scopes, time.Now().Add(time.Hour))
				must(t, e)
				return token
			}
			all := []string{"account:read", "account:write"}
			rest, mcpToken := grant("rest", all), grant("mcp", all)
			path := "/api/v1/" + kind + "-account/profile"
			mcpPath := "/mcp/account/" + kind
			call := func(token, name string, args map[string]any) json.RawMessage {
				return mcpRequestAt(t, server, mcpPath, token, "tools/call", map[string]any{"name": name, "arguments": args}, 200).Result
			}
			f.request(t, "GET", path, mcpToken, "", "", 401)
			f.request(t, "GET", path, f.token, "", "", 401)
			mcpRequestAt(t, server, mcpPath, rest, "tools/list", map[string]any{}, 401)
			opposite := "merchant"
			if kind == "merchant" {
				opposite = "customer"
			}
			mcpRequestAt(t, server, "/mcp/account/"+opposite, mcpToken, "tools/list", map[string]any{}, 401)
			mcpRequestAt(t, server, "/mcp/merchant", mcpToken, "tools/list", map[string]any{}, 401)
			f.request(t, "GET", f.merchantPath, rest, "", "", 401)
			initial := f.request(t, "GET", path, rest, "", "", 200)
			if bytes.Contains(initial, []byte("token")) || bytes.Contains(initial, []byte("merchant_id")) {
				t.Fatal("profile leaked credential/business scope")
			}
			inspected := toolOutput(t, call(mcpToken, "inspect_my_profile", map[string]any{}))
			if inspected["id"] != id.String() || inspected["revision"] != float64(1) {
				t.Fatal(inspected)
			}
			args := map[string]any{"expected_revision": 1, "display_name": "Owner updated", "locale": "ar", "request_key": "account-mcp-request-0001"}
			first := toolOutput(t, call(mcpToken, "update_my_preferences", args))
			if first["replayed"] != false || first["profile"].(map[string]any)["revision"] != float64(2) {
				t.Fatal(first)
			}
			if toolOutput(t, call(mcpToken, "update_my_preferences", args))["replayed"] != true {
				t.Fatal("missing replay")
			}
			args["display_name"] = "Changed request"
			toolCode(t, call(mcpToken, "update_my_preferences", args), "idempotency_conflict")
			args["request_key"] = "account-mcp-request-0002"
			toolCode(t, call(mcpToken, "update_my_preferences", args), "account_revision_conflict")
			after := f.request(t, "GET", path, rest, "", "", 200)
			if !bytes.Contains(after, []byte(`"locale":"ar"`)) {
				t.Fatal(string(after))
			}
			body := `{"expected_revision":2,"display_name":"REST update","locale":"de"}`
			changed := f.request(t, "PUT", path, rest, "account-rest-request-0001", body, 200)
			if !bytes.Equal(changed, f.request(t, "PUT", path, rest, "account-rest-request-0001", body, 200)) {
				t.Fatal("REST replay changed")
			}
			f.request(t, "PUT", path, rest, "account-rest-request-0002", body, 409)
			f.request(t, "PUT", path, rest, "account-rest-request-0003", `{"expected_revision":3,"display_name":"","locale":"fr"}`, 400)
			readOnly := grant("mcp", []string{"account:read"})
			listed := mcpRequestAt(t, server, mcpPath, readOnly, "tools/list", map[string]any{}, 200).Result
			if bytes.Contains(listed, []byte("update_my_preferences")) {
				t.Fatal("write tool disclosed to read-only grant")
			}
			readonlyREST := grant("rest", []string{"account:read"})
			f.request(t, "PUT", path, readonlyREST, "account-rest-request-0004", body, 403)
			// RLS protects other accounts even when SQL is executed directly as runtime.
			other, e := core.ProvisionAccount(ctx, f.admin, kind, "Other owner", mid)
			must(t, e)
			tx, e := database.AccountTransaction(ctx, f.app, id)
			must(t, e)
			var count int
			must(t, tx.QueryRowContext(ctx, "SELECT count(*) FROM auth.accounts WHERE id=$1", other).Scan(&count))
			must(t, tx.Rollback())
			if count != 0 {
				t.Fatal("cross-account RLS leak")
			}
			// A fresh authentication and transaction check rejects revoked credentials,
			// including a formerly successful retry.
			p, e := core.AuthenticateAccount(ctx, f.app, mcpToken, core.AccountResource(kind, "mcp"))
			must(t, e)
			_, e = f.admin.ExecContext(ctx, "UPDATE auth.account_grants SET revoked=true WHERE id=$1", p.ID)
			must(t, e)
			mcpRequestAt(t, server, mcpPath, mcpToken, "tools/list", map[string]any{}, 401)
			if _, e = (commerce.Accounts{DB: f.app}).Inspect(ctx, p); e == nil {
				t.Fatal("retained principal bypassed revocation")
			}
			var audit int
			must(t, f.admin.QueryRowContext(ctx, "SELECT count(*) FROM auth.account_audit WHERE account_id=$1", id).Scan(&audit))
			if audit != 2 {
				t.Fatalf("audit count %d", audit)
			}
		})
	}
}
func TestAccountConcurrentRetry(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	id, err := core.ProvisionAccount(ctx, f.admin, "customer", "Example", uuid.Nil)
	must(t, err)
	token, err := core.ProvisionAccountGrant(ctx, f.admin, id, "customer", "rest", []string{"account:read", "account:write"}, time.Now().Add(time.Hour))
	must(t, err)
	p, err := core.AuthenticateAccount(ctx, f.app, token, "customer-account-rest")
	must(t, err)
	var wg sync.WaitGroup
	results := make(chan core.Result, 2)
	failures := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			r, e := (commerce.Accounts{DB: f.app}).Update(ctx, p, 1, "Updated", "de", "concurrent-account-0001", uuid.New())
			if e != nil {
				failures <- e
			} else {
				results <- r
			}
		})
	}
	wg.Wait()
	close(results)
	close(failures)
	for e := range failures {
		t.Fatal(e)
	}
	replayed := 0
	var body []byte
	for r := range results {
		if r.Replayed {
			replayed++
		}
		if body != nil && !bytes.Equal(body, r.Body) {
			t.Fatal("different retry result")
		}
		body = r.Body
	}
	if replayed != 1 {
		t.Fatalf("replays=%d", replayed)
	}
}
