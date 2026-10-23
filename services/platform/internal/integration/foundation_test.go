// Package integration exercises the maintained HTTP and SQL boundaries together.
package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"ocp.local/platform/internal/infra/database"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/httpapi"
)

type fixture struct {
	admin, app   *sql.DB
	server       *httptest.Server
	router       routers.Router
	principal    persistence.Principal
	token        string
	merchantPath string
}

func setup(t *testing.T) *fixture {
	return setupAtVersion(t, 0)
}

// A bounded historical version supports real upgrade tests, not simulated DDL.
func setupAtVersion(t *testing.T, version int64) *fixture {
	t.Helper()
	dsn := os.Getenv("OCP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires task test:integration (disposable PostgreSQL)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	root, err := database.Open(ctx, dsn)
	must(t, err)
	t.Cleanup(func() { root.Close() })
	name := "foundation_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = root.ExecContext(ctx, `CREATE DATABASE `+name)
	must(t, err)
	t.Cleanup(func() {
		_, err := root.Exec(`DROP DATABASE ` + name + ` WITH (FORCE)`)
		if err != nil {
			t.Errorf("cleanup isolated database: %v", err)
		}
	})
	parsed, err := url.Parse(dsn)
	must(t, err)
	parsed.Path = "/" + name
	admin, err := database.Open(ctx, parsed.String())
	must(t, err)
	t.Cleanup(func() { admin.Close() })
	if version == 0 {
		must(t, database.Migrate(ctx, admin))
		must(t, database.Migrate(ctx, admin))
	} else {
		provider, err := goose.NewProvider(goose.DialectPostgres, admin, os.DirFS("../infra/database/migrations"))
		must(t, err)
		_, err = provider.UpTo(ctx, version)
		must(t, err)
	}
	parsed.User = url.UserPassword("ocp_app", "ocp_app_local_only")
	app, err := database.Open(ctx, parsed.String())
	must(t, err)
	t.Cleanup(func() { app.Close() })
	must(t, database.CheckRuntimeRole(ctx, app))
	if database.CheckRuntimeRole(ctx, admin) == nil {
		t.Fatal("administrative runtime role accepted")
	}
	p, token, err := persistence.Provision(ctx, admin, "Example restaurant", []string{"store:read", "store:write", "offering:read", "offering:write", "order:read", "order:manage"}, time.Now().Add(time.Hour))
	must(t, err)
	handler, err := httpapi.NewHandler(app, httpapi.WithDevelopmentPayments())
	must(t, err)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	spec, err := httpapi.GetSwagger()
	must(t, err)
	router, err := legacy.NewRouter(spec)
	must(t, err)
	return &fixture{admin, app, server, router, p, token, "/api/v1/merchants/" + p.MerchantID.String() + "/stores"}
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) request(t *testing.T, method, path, token, key, body string, want int) []byte {
	t.Helper()
	req, err := http.NewRequest(method, f.server.URL+path, strings.NewReader(body))
	must(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := f.server.Client().Do(req)
	must(t, err)
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	must(t, err)
	if response.StatusCode != want {
		t.Fatalf("%s %s: want %d got %d: %s", method, path, want, response.StatusCode, data)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("private response is cacheable")
	}
	requestID := response.Header.Get("X-Request-ID")
	if _, err := uuid.Parse(requestID); err != nil {
		t.Fatal("missing/invalid request identifier")
	}
	// Mirror the relative request URL received by the HTTP server. The contract
	// advertises '/' rather than this test server's random absolute address.
	validationRequest := req.Clone(req.Context())
	relativeURL := *req.URL
	relativeURL.Scheme = ""
	relativeURL.Host = ""
	validationRequest.URL = &relativeURL
	route, params, err := f.router.FindRoute(validationRequest)
	must(t, err)
	input := &openapi3filter.ResponseValidationInput{RequestValidationInput: &openapi3filter.RequestValidationInput{Request: validationRequest, PathParams: params, Route: route}, Status: response.StatusCode, Header: response.Header}
	input.SetBodyBytes(data)
	must(t, openapi3filter.ValidateResponse(context.Background(), input))
	if want >= 400 {
		var problem httpapi.Problem
		must(t, json.Unmarshal(data, &problem))
		if problem.Code == "" {
			t.Fatal("missing problem code")
		}
		if problem.Instance != "urn:uuid:"+requestID {
			t.Fatal("problem instance does not match request identifier")
		}
		if want == 401 && response.Header.Get("WWW-Authenticate") == "" {
			t.Fatal("missing bearer challenge")
		}
	}
	return data
}
func menu(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile("../../../../contracts/openapi/v1/examples/food-menu.json")
	must(t, err)
	var value map[string]any
	must(t, json.Unmarshal(data, &value))
	return value
}
func offeringBody(t *testing.T, base int64, doc map[string]any, model string) string {
	t.Helper()
	value := map[string]any{"base_revision": base, "model": map[string]any{"id": model, "version": "1.0.0"}, "document": doc}
	data, err := json.Marshal(value)
	must(t, err)
	return string(data)
}

func TestFoundation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	var store persistence.Store
	t.Run("generated_client_creates_store", func(t *testing.T) {
		client, err := httpapi.NewClientWithResponses(f.server.URL, httpapi.WithRequestEditorFn(func(ctx context.Context, r *http.Request) error {
			r.Header.Set("Authorization", "Bearer "+f.token)
			return nil
		}))
		must(t, err)
		response, err := client.CreateStoreWithResponse(ctx, f.principal.MerchantID, &httpapi.CreateStoreParams{IdempotencyKey: "store-generated-0001"}, httpapi.CreateStoreJSONRequestBody{DisplayName: "Example Döner"})
		must(t, err)
		if response.StatusCode() != 201 || response.JSON201 == nil {
			t.Fatalf("generated client: %d %s", response.StatusCode(), response.Body)
		}
		must(t, json.Unmarshal(response.Body, &store))
	})
	storePath := f.merchantPath + "/" + store.ID.String()
	offeringPath := storePath + "/offering"
	t.Run("read_store", func(t *testing.T) { f.request(t, "GET", storePath, f.token, "", "", 200) })
	t.Run("liveness_is_not_readiness", func(t *testing.T) { f.request(t, "GET", "/health/live", "", "", "", 200) })
	t.Run("missing_and_invalid_credentials", func(t *testing.T) {
		for _, token := range []string{"", "garbage", "ocp_" + strings.Repeat("x", 43)} {
			f.request(t, "GET", storePath, token, "", "", 401)
		}
	})
	t.Run("tenant_and_action_grants", func(t *testing.T) {
		other, token, err := persistence.Provision(ctx, f.admin, "Other restaurant", []string{"store:read"}, time.Now().Add(time.Hour))
		must(t, err)
		f.request(t, "GET", storePath, token, "", "", 403)
		f.request(t, "GET", "/api/v1/merchants/"+other.MerchantID.String()+"/stores/"+store.ID.String(), token, "", "", 404)
		f.request(t, "POST", "/api/v1/merchants/"+other.MerchantID.String()+"/stores", token, "read-only-attempt-01", `{"display_name":"No"}`, 403)
	})
	t.Run("expired_and_revoked_credentials", func(t *testing.T) {
		_, token, err := persistence.Provision(ctx, f.admin, "Expired", []string{"store:read"}, time.Now().Add(-time.Minute))
		must(t, err)
		f.request(t, "GET", storePath, token, "", "", 401)
		p, token, err := persistence.Provision(ctx, f.admin, "Revoked", []string{"store:write"}, time.Now().Add(time.Hour))
		must(t, err)
		path := "/api/v1/merchants/" + p.MerchantID.String() + "/stores"
		f.request(t, "POST", path, token, "revocation-replay-01", `{"display_name":"Revoked replay"}`, 201)
		_, err = f.admin.Exec(`UPDATE auth.grants SET revoked=true WHERE id=$1`, p.ID)
		must(t, err)
		f.request(t, "POST", path, token, "revocation-replay-01", `{"display_name":"Revoked replay"}`, 401)
	})
	t.Run("bounded_structural_requests", func(t *testing.T) {
		for i, body := range []string{`{`, `{"display_name":"Valid","extra":true}`, `{"display_name":"   "}`, `{"display_name":"X"} {}`} {
			f.request(t, "POST", f.merchantPath, f.token, fmt.Sprintf("invalid-request-%03d", i), body, 400)
		}
		f.request(t, "POST", f.merchantPath, f.token, "", `{"display_name":"Missing key"}`, 400)
		f.request(t, "POST", f.merchantPath, f.token, "bad", `{"display_name":"Short key"}`, 400)
		f.request(t, "GET", f.merchantPath+"/not-a-uuid", f.token, "", "", 400)
		f.request(t, "POST", f.merchantPath, f.token, "oversized-request-01", `{"display_name":"`+strings.Repeat("x", 262144)+`"}`, 413)
		req, err := http.NewRequest("POST", f.server.URL+f.merchantPath, strings.NewReader(`{"display_name":"Wrong type"}`))
		must(t, err)
		req.Header.Set("Authorization", "Bearer "+f.token)
		req.Header.Set("Idempotency-Key", "wrong-media-type-01")
		req.Header.Set("Content-Type", "text/plain")
		response, err := f.server.Client().Do(req)
		must(t, err)
		defer response.Body.Close()
		if response.StatusCode != 415 {
			t.Fatalf("wrong media: %d", response.StatusCode)
		}
		var problem httpapi.Problem
		must(t, json.NewDecoder(response.Body).Decode(&problem))
		if problem.Code != "unsupported_media_type" || problem.Instance != "urn:uuid:"+response.Header.Get("X-Request-ID") {
			t.Fatal("invalid media error")
		}
	})
	t.Run("replay_preserves_original_body", func(t *testing.T) {
		original := f.request(t, "POST", f.merchantPath, f.token, "reordered-replay-01", `{"display_name":"Replay"}`, 201)
		replay := f.request(t, "POST", f.merchantPath, f.token, "reordered-replay-01", "{ \n \"display_name\" : \"Replay\" }", 201)
		if !bytes.Equal(original, replay) {
			t.Fatal("replay changed response bytes")
		}
		f.request(t, "POST", f.merchantPath, f.token, "reordered-replay-01", `{"display_name":"Changed"}`, 409)
	})
	t.Run("offering_initial_revision", func(t *testing.T) {
		f.request(t, "GET", offeringPath, f.token, "", "", 404)
		f.request(t, "PUT", offeringPath, f.token, "initial-offering-01", offeringBody(t, 0, menu(t), "ocp.food.menu"), 201)
		f.request(t, "GET", offeringPath, f.token, "", "", 200)
	})
	t.Run("unknown_and_invalid_models", func(t *testing.T) {
		f.request(t, "PUT", offeringPath, f.token, "unknown-model-0001", offeringBody(t, 1, menu(t), "future.model"), 422)
		doc := menu(t)
		doc["items"].([]any)[0].(map[string]any)["option_groups"].([]any)[0].(map[string]any)["min_selected"] = 2
		f.request(t, "PUT", offeringPath, f.token, "invalid-menu-00001", offeringBody(t, 1, doc, "ocp.food.menu"), 422)
	})
	t.Run("canonical_offering_replay", func(t *testing.T) {
		original := f.request(t, "PUT", offeringPath, f.token, "initial-offering-01", offeringBody(t, 0, menu(t), "ocp.food.menu"), 201)
		var body any
		must(t, json.Unmarshal([]byte(offeringBody(t, 0, menu(t), "ocp.food.menu")), &body))
		pretty, err := json.MarshalIndent(body, "", " ")
		must(t, err)
		replay := f.request(t, "PUT", offeringPath, f.token, "initial-offering-01", string(pretty), 201)
		if !bytes.Equal(original, replay) {
			t.Fatal("offering replay changed bytes")
		}
	})
	t.Run("concurrent_revision_writers", func(t *testing.T) {
		body := offeringBody(t, 1, menu(t), "ocp.food.menu")
		var wg sync.WaitGroup
		statuses := make(chan int, 2)
		for i := range 2 {
			wg.Go(func() {
				req, err := http.NewRequest("PUT", f.server.URL+offeringPath, strings.NewReader(body))
				if err != nil {
					statuses <- 0
					return
				}
				req.Header.Set("Authorization", "Bearer "+f.token)
				req.Header.Set("Idempotency-Key", fmt.Sprintf("concurrent-revision-%d", i))
				req.Header.Set("Content-Type", "application/json")
				res, err := f.server.Client().Do(req)
				if err != nil {
					statuses <- 0
					return
				}
				defer res.Body.Close()
				io.Copy(io.Discard, res.Body)
				statuses <- res.StatusCode
			})
		}
		wg.Wait()
		close(statuses)
		counts := map[int]int{}
		for status := range statuses {
			counts[status]++
		}
		if counts[201] != 1 || counts[409] != 1 {
			t.Fatalf("concurrent results: %v", counts)
		}
		f.request(t, "GET", offeringPath+"/revisions/1", f.token, "", "", 200)
		current := f.request(t, "PUT", offeringPath, f.token, "stale-revision-001", body, 409)
		if !bytes.Contains(current, []byte(`"current_revision":2`)) {
			t.Fatalf("missing current revision: %s", current)
		}
	})
	t.Run("concurrent_same_key_one_mutation", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan string, 4)
		for range 4 {
			wg.Go(func() {
				req, _ := http.NewRequest("POST", f.server.URL+f.merchantPath, strings.NewReader(`{"display_name":"Concurrent"}`))
				req.Header.Set("Authorization", "Bearer "+f.token)
				req.Header.Set("Idempotency-Key", "same-key-concurrent-01")
				req.Header.Set("Content-Type", "application/json")
				res, err := f.server.Client().Do(req)
				if err != nil {
					results <- "failure"
					return
				}
				defer res.Body.Close()
				data, _ := io.ReadAll(res.Body)
				results <- fmt.Sprintf("%d:%s", res.StatusCode, data)
			})
		}
		wg.Wait()
		close(results)
		first := ""
		for value := range results {
			if first == "" {
				first = value
			}
			if value != first || !strings.HasPrefix(value, "201:") {
				t.Fatalf("nonidentical replay: %s", value)
			}
		}
		var count int
		must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.stores WHERE display_name='Concurrent'`).Scan(&count))
		if count != 1 {
			t.Fatal("duplicate mutation")
		}
	})
	t.Run("atomic_mutation_rollback", func(t *testing.T) {
		_, err := persistence.Mutate(ctx, f.app, f.principal, "createStore", f.principal.MerchantID.String(), "rollback-key-0001", []byte(`{}`), func(tx *sql.Tx) (any, error) {
			_, err := persistence.CreateStore(ctx, tx, f.principal.MerchantID, "Must roll back")
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("forced failure")
		})
		if err == nil {
			t.Fatal("failure lost")
		}
		var count int
		must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.stores WHERE display_name='Must roll back'`).Scan(&count))
		if count != 0 {
			t.Fatal("partial mutation committed")
		}
		must(t, f.admin.QueryRow(`SELECT count(*) FROM commerce.request_replays WHERE replay_key='rollback-key-0001'`).Scan(&count))
		if count != 0 {
			t.Fatal("partial replay committed")
		}
	})
	t.Run("forced_RLS_and_pool_cleanup", func(t *testing.T) {
		f.app.SetMaxOpenConns(1)
		f.app.SetMaxIdleConns(1)
		tx, err := database.TenantTransaction(ctx, f.app, uuid.New())
		must(t, err)
		var count int
		must(t, tx.QueryRow(`SELECT count(*) FROM commerce.stores`).Scan(&count))
		if count != 0 {
			t.Fatal("cross-tenant rows visible")
		}
		_, err = persistence.CreateStore(ctx, tx, f.principal.MerchantID, "Wrong tenant")
		if err == nil {
			t.Fatal("cross-tenant insert allowed")
		}
		must(t, tx.Rollback())
		for _, commit := range []bool{false, true} {
			tx, err := database.TenantTransaction(ctx, f.app, f.principal.MerchantID)
			must(t, err)
			if commit {
				must(t, tx.Commit())
			} else {
				must(t, tx.Rollback())
			}
			must(t, f.app.QueryRow(`SELECT count(*) FROM commerce.stores`).Scan(&count))
			if count != 0 {
				t.Fatal("tenant context leaked through pool")
			}
		}
		tx, err = database.TenantTransaction(ctx, f.app, f.principal.MerchantID)
		must(t, err)
		_, err = tx.Exec(`UPDATE commerce.offering_revisions SET revision=99`)
		if err == nil {
			t.Fatal("immutable revision updated")
		}
		must(t, tx.Rollback())
		if err = f.app.QueryRow(`SELECT count(*) FROM auth.grants`).Scan(&count); err == nil {
			t.Fatal("credential table readable")
		}
	})
	t.Run("handler_restart_retains_data", func(t *testing.T) {
		f.server.Close()
		handler, err := httpapi.NewHandler(f.app)
		must(t, err)
		f.server = httptest.NewServer(handler)
		defer f.server.Close()
		f.request(t, "GET", storePath, f.token, "", "", 200)
		f.request(t, "GET", offeringPath+"/revisions/1", f.token, "", "", 200)
	})
	t.Run("credentials_stored_as_digests", func(t *testing.T) {
		var length int
		must(t, f.admin.QueryRow(`SELECT octet_length(token_hash) FROM auth.grants WHERE id=$1`, f.principal.ID).Scan(&length))
		if length != 32 {
			t.Fatal("invalid credential digest")
		}
	})
	t.Run("runtime_role_rejects_owner_and_elevation", func(t *testing.T) {
		var adminRole string
		must(t, f.admin.QueryRow(`SELECT current_user`).Scan(&adminRole))
		// The fixture chooses a known administrative role in its disposable cluster.
		// Quote the returned identifier before interpolating SQL object names.
		quoted := `"` + strings.ReplaceAll(adminRole, `"`, `""`) + `"`
		_, err := f.admin.Exec(`ALTER TABLE commerce.stores OWNER TO ocp_app`)
		must(t, err)
		defer func() { _, err := f.admin.Exec(`ALTER TABLE commerce.stores OWNER TO ` + quoted); must(t, err) }()
		if database.CheckRuntimeRole(ctx, f.app) == nil {
			t.Fatal("table-owning runtime accepted")
		}
		_, err = f.admin.Exec(`ALTER TABLE commerce.stores OWNER TO ` + quoted)
		must(t, err)
		_, err = f.admin.Exec(`ALTER TABLE payment.development_operations OWNER TO ocp_app`)
		must(t, err)
		defer func() {
			_, err := f.admin.Exec(`ALTER TABLE payment.development_operations OWNER TO ` + quoted)
			must(t, err)
		}()
		if database.CheckRuntimeRole(ctx, f.app) == nil {
			t.Fatal("payment-table-owning runtime accepted")
		}
		_, err = f.admin.Exec(`ALTER TABLE payment.development_operations OWNER TO ` + quoted)
		must(t, err)
		_, err = f.admin.Exec(`ALTER SCHEMA payment OWNER TO ocp_app`)
		must(t, err)
		defer func() { _, err := f.admin.Exec(`ALTER SCHEMA payment OWNER TO ` + quoted); must(t, err) }()
		if database.CheckRuntimeRole(ctx, f.app) == nil {
			t.Fatal("payment-schema-owning runtime accepted")
		}
		_, err = f.admin.Exec(`ALTER SCHEMA payment OWNER TO ` + quoted)
		must(t, err)
		_, err = f.admin.Exec(`GRANT ` + quoted + ` TO ocp_app`)
		must(t, err)
		defer func() { _, err := f.admin.Exec(`REVOKE ` + quoted + ` FROM ocp_app`); must(t, err) }()
		if database.CheckRuntimeRole(ctx, f.app) == nil {
			t.Fatal("administratively elevated runtime accepted")
		}
		_, err = f.admin.Exec(`REVOKE ` + quoted + ` FROM ocp_app`)
		must(t, err)
		must(t, database.CheckRuntimeRole(ctx, f.app))
	})
	t.Run("credential_lookup_security_definer_boundary", func(t *testing.T) {
		var safe bool
		must(t, f.app.QueryRow(`SELECT p.prosecdef
   AND p.proowner <> current_user::regrole
   AND p.proconfig @> ARRAY['search_path=pg_catalog']
   AND has_function_privilege(current_user,p.oid,'EXECUTE')
   AND NOT EXISTS (SELECT FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) acl
     WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE')
   FROM pg_proc p WHERE p.oid='auth.lookup_grant(bytea)'::regprocedure`).Scan(&safe))
		if !safe {
			t.Fatal("unsafe credential lookup ownership/ACL/search path")
		}
	})
}
