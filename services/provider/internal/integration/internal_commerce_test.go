package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"ocp.local/platform/internal/database"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/sessionkit"
	"ocp.local/platform/internal/ucpcontract"
	"ocp.local/platform/internal/ucpinternal"
)

type internalFixture struct {
	f                                               *fixture
	server                                          *httptest.Server
	shop, shopOther, shopManage, hotel, hotelManage string
	shopPrincipal                                   sessionkit.Principal
	storeID                                         uuid.UUID
	apiURL                                          string
}

const internalProfile = "https://agent.example/.well-known/ucp"
const payLater = `{"payment":{"instruments":[{"id":"pay-later","handler_id":"ocp-pay-later","type":"pay_later"}]}}`
const shopCatalog = `{"base_revision":0,"document":{"currency":"EUR","payment_condition":"pay_later","products":[{"id":"shirt","title":"Shirt","variants":[{"id":"shirt-blue","title":"Blue shirt","price_minor":2500,"stock":1},{"id":"shirt-red","title":"Red shirt","price_minor":2600,"stock":5}]}]}}`
const hotelCatalog = `{"base_revision":0,"document":{"currency":"EUR","payment_condition":"pay_later","time_zone":"Europe/Berlin","property":{"id":"hotel","name":"Example hotel"},"units":[{"id":"room-1","title":"Room one","capacity":2,"rates":[{"id":"flex","title":"Flexible","nightly_price_minor":10000}]}]}}`
const shoppingSelection = `{"line_items":[{"item":{"id":"shirt-blue"},"quantity":1}]}`

func internalSetup(t *testing.T) *internalFixture {
	t.Helper()
	f := setup(t)
	store := preparedStore(t, f)
	h, err := ucpinternal.NewHandler(f.app, ucpinternal.Config{Endpoint: "https://internal.ocplab.org", Concurrency: 2})
	must(t, err)
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	x := &internalFixture{f: f, server: server, storeID: store.ID}
	provision := func(d capability.Domain, customer uuid.UUID, management bool) (sessionkit.Principal, string) {
		p, token, err := ucpinternal.Provision(context.Background(), f.admin, sessionkit.Principal{MerchantID: f.principal.MerchantID, StoreID: store.ID, CustomerID: customer, Domain: d, Profile: internalProfile}, ucpinternal.ControlledProfile(capability.Shopping, capability.Lodging), management, time.Now().Add(time.Hour))
		must(t, err)
		return p, token
	}
	x.shopPrincipal, x.shop = provision(capability.Shopping, uuid.New(), false)
	_, x.shopOther = provision(capability.Shopping, uuid.New(), false)
	_, x.shopManage = provision(capability.Shopping, uuid.New(), true)
	_, x.hotel = provision(capability.Lodging, uuid.New(), false)
	_, x.hotelManage = provision(capability.Lodging, uuid.New(), true)
	return x
}
func (x *internalFixture) request(t *testing.T, method, path, token, key, body string, status int) []byte {
	t.Helper()
	base := x.server.URL
	if x.apiURL != "" {
		base = x.apiURL
	}
	req, err := http.NewRequest(method, base+path, strings.NewReader(body))
	must(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("UCP-Agent", `profile="`+internalProfile+`"`)
	req.Header.Set("Request-Id", uuid.NewString())
	req.Header.Set("Accept", "application/json")
	if method != "GET" {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := http.DefaultClient.Do(req)
	must(t, err)
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	must(t, err)
	if res.StatusCode != status {
		t.Fatalf("%s %s expected %d got %d: %s", method, path, status, res.StatusCode, raw)
	}
	if _, err := uuid.Parse(res.Header.Get("Request-Id")); err != nil {
		t.Fatal("missing UCP Request-Id")
	}
	if status >= 400 && !strings.HasPrefix(res.Header.Get("Content-Type"), "application/problem+json") {
		t.Fatal("missing problem media type")
	}
	spec, err := ucpcontract.GetSwagger()
	must(t, err)
	spec.Servers = nil
	router, err := legacy.NewRouter(spec)
	must(t, err)
	validation := req.Clone(context.Background())
	relative := *req.URL
	relative.Host = ""
	relative.Scheme = ""
	validation.URL = &relative
	route, params, err := router.FindRoute(validation)
	must(t, err)
	output := &openapi3filter.ResponseValidationInput{RequestValidationInput: &openapi3filter.RequestValidationInput{Request: validation, PathParams: params, Route: route}, Status: status, Header: res.Header}
	output.SetBodyBytes(raw)
	must(t, openapi3filter.ValidateResponse(context.Background(), output))
	if status < 300 && strings.Contains(path, "-sessions") {
		var root map[string]any
		must(t, json.Unmarshal(raw, &root))
		if root["id"] != nil {
			domain := "shopping"
			if strings.HasPrefix(path, "/lodging/") {
				domain = "lodging"
			}
			validateUpstreamResponse(t, domain, raw)
		}
	}
	return raw
}
func sessionID(t *testing.T, raw []byte) string {
	t.Helper()
	var b sessionkit.Base
	must(t, json.Unmarshal(raw, &b))
	if b.ID == uuid.Nil {
		t.Fatal("missing session id")
	}
	return b.ID.String()
}
func sessionStatus(t *testing.T, raw []byte) string {
	t.Helper()
	var b sessionkit.Base
	must(t, json.Unmarshal(raw, &b))
	if b.UCP.Version != sessionkit.Version || b.UCP.PaymentHandlers == nil {
		t.Fatal("missing UCP metadata")
	}
	return b.Status
}
func TestInternalShoppingJourney(t *testing.T) {
	x := internalSetup(t)
	x.request(t, "PUT", "/shopping/catalog", x.shopManage, uuid.NewString(), shopCatalog, 200)
	profile := x.request(t, "GET", "/.well-known/ucp", x.shop, "", "", 200)
	if !bytes.Contains(profile, []byte("dev.ucp.shopping.checkout")) || bytes.Contains(profile, []byte("dev.ucp.lodging.booking")) {
		t.Fatal("incorrect advertised domain")
	}
	var advertised ucpinternal.Profile
	must(t, json.Unmarshal(profile, &advertised))
	if advertised.UCP.PaymentHandlers[sessionkit.PayLaterHandler][0].Schema != "https://internal.ocplab.org/schemas/internal/pay-later.json" {
		t.Fatal("incorrect schema origin")
	}
	x.request(t, "GET", "/schemas/internal/pay-later.json", x.shop, "", "", 200)
	key := uuid.NewString()
	first := x.request(t, "POST", "/shopping/checkout-sessions", x.shop, key, shoppingSelection, 201)
	id := sessionID(t, first)
	if sessionStatus(t, first) != "ready_for_complete" {
		t.Fatal("checkout not ready")
	}
	reordered := `{"line_items":[{"quantity":1,"item":{"id":"shirt-blue"}}]}`
	if !bytes.Equal(first, x.request(t, "POST", "/shopping/checkout-sessions", x.shop, key, reordered, 201)) {
		t.Fatal("canonical replay changed")
	}
	x.request(t, "POST", "/shopping/checkout-sessions", x.shop, key, strings.Replace(shoppingSelection, "shirt-blue", "shirt-red", 1), 409)
	path := "/shopping/checkout-sessions/" + id
	x.request(t, "GET", path, x.shopOther, "", "", 404)
	x.request(t, "GET", path, x.hotel, "", "", 404)
	x.request(t, "PUT", "/shopping/catalog", x.shop, uuid.NewString(), shopCatalog, 403)
	x.request(t, "POST", path+"/complete", x.shop, uuid.NewString(), `{}`, 400)
	completeKey := uuid.NewString()
	completed := x.request(t, "POST", path+"/complete", x.shop, completeKey, payLater, 200)
	if sessionStatus(t, completed) != "completed" {
		t.Fatalf("checkout not complete: %s", completed)
	}
	var confirmation struct {
		Order struct {
			ID        string `json:"id"`
			Permalink string `json:"permalink_url"`
		} `json:"order"`
	}
	must(t, json.Unmarshal(completed, &confirmation))
	orderPath := "/shopping/orders/" + confirmation.Order.ID
	if confirmation.Order.Permalink != "https://internal.ocplab.org"+orderPath {
		t.Fatal("incorrect order permalink")
	}
	x.request(t, "GET", orderPath, x.shop, "", "", 200)
	x.request(t, "GET", orderPath, x.shopOther, "", "", 404)
	storeTx, err := database.TenantTransaction(context.Background(), x.f.app, x.f.principal.MerchantID)
	must(t, err)
	otherStore, err := persistence.CreateStore(context.Background(), storeTx, x.f.principal.MerchantID, "Another store")
	must(t, err)
	must(t, storeTx.Commit())
	otherPrincipal := x.shopPrincipal
	otherPrincipal.StoreID = otherStore.ID
	_, otherToken, err := ucpinternal.Provision(context.Background(), x.f.admin, otherPrincipal, ucpinternal.ControlledProfile(capability.Shopping), false, time.Now().Add(time.Hour))
	must(t, err)
	x.request(t, "GET", orderPath, otherToken, "", "", 404)
	// Recreate the HTTP server: neither session state nor replay depends on memory.
	h, err := ucpinternal.NewHandler(x.f.app, ucpinternal.Config{Endpoint: "https://internal.ocplab.org", Concurrency: 2})
	must(t, err)
	replica := httptest.NewServer(h)
	defer replica.Close()
	old := x.server
	x.server = replica
	if !bytes.Equal(completed, x.request(t, "POST", path+"/complete", x.shop, completeKey, payLater, 200)) {
		t.Fatal("restart replay changed")
	}
	x.server = old
	if sessionStatus(t, x.request(t, "POST", path+"/cancel", x.shop, uuid.NewString(), `{}`, 200)) != "completed" {
		t.Fatal("session cancel changed a completed order")
	}
	var orders, obligations int
	var available int64
	must(t, x.f.admin.QueryRow(`SELECT count(*) FROM shopping.orders`).Scan(&orders))
	must(t, x.f.admin.QueryRow(`SELECT count(*) FROM payments.obligations WHERE state='outstanding' AND condition='pay_later' AND amount_minor=2500`).Scan(&obligations))
	must(t, x.f.admin.QueryRow(`SELECT available FROM shopping.stock WHERE sku='shirt-blue'`).Scan(&available))
	if orders != 1 || obligations != 1 || available != 0 {
		t.Fatalf("orders=%d obligations=%d stock=%d", orders, obligations, available)
	}
	// Revocation/expiry applies even when a successful replay already exists.
	_, err = x.f.admin.Exec(`UPDATE commerce_internal.grants SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, x.shopPrincipal.ID)
	must(t, err)
	x.request(t, "POST", path+"/complete", x.shop, completeKey, payLater, 401)
}

func TestInternalAdmissionAndCredentialPrivileges(t *testing.T) {
	x := internalSetup(t)
	ctx := context.Background()
	must(t, database.CheckRuntimeRole(ctx, x.f.app))
	var hardened bool
	must(t, x.f.admin.QueryRow(`SELECT p.prosecdef AND r.rolname='ocp_internal_auth_owner' AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolbypassrls
 AND p.proconfig=ARRAY['search_path=pg_catalog'] AND NOT EXISTS(SELECT FROM aclexplode(p.proacl) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')
 AND NOT pg_has_role('ocp_app',r.oid,'MEMBER') AND pg_get_function_result(p.oid) NOT LIKE '%token_hash%'
 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid='commerce_internal.lookup_grant(bytea)'::regprocedure`).Scan(&hardened))
	if !hardened {
		t.Fatal("internal authentication definer not hardened")
	}
	if _, err := x.f.app.Exec(`SELECT token_hash FROM commerce_internal.grants`); err == nil {
		t.Fatal("runtime can read credential digests")
	}
	if _, err := x.f.app.Exec(`SET ROLE ocp_internal_auth_owner`); err == nil {
		t.Fatal("runtime can become authentication owner")
	}
	for _, profile := range []ucpinternal.Profile{ucpinternal.ControlledProfile(capability.Lodging), ucpinternal.ControlledProfile()} {
		if _, _, err := ucpinternal.Provision(ctx, x.f.admin, x.shopPrincipal, profile, false, time.Now().Add(time.Hour)); err == nil {
			t.Fatal("bound capability missing but credential issued")
		}
	}
	profile := ucpinternal.ControlledProfile(capability.Shopping)
	wrongURI := x.shopPrincipal
	wrongURI.Profile = "https://other.example/.well-known/ucp"
	if _, _, err := ucpinternal.Provision(ctx, x.f.admin, wrongURI, profile, false, time.Now().Add(time.Hour)); err == nil {
		t.Fatal("arbitrary platform profile URI admitted")
	}
	profile.UCP.Services["untrusted.service"] = []sessionkit.Entity{{Version: sessionkit.Version, Endpoint: "http://127.0.0.1"}}
	if _, _, err := ucpinternal.Provision(ctx, x.f.admin, x.shopPrincipal, profile, false, time.Now().Add(time.Hour)); err == nil {
		t.Fatal("arbitrary service admitted")
	}
	profile = ucpinternal.ControlledProfile(capability.Shopping)
	profile.UCP.PaymentHandlers[sessionkit.PayLaterHandler][0].ID = "different-handler"
	if _, _, err := ucpinternal.Provision(ctx, x.f.admin, x.shopPrincipal, profile, false, time.Now().Add(time.Hour)); err == nil {
		t.Fatal("unselected payment handler admitted")
	}
	for _, table := range []string{"shopping.sessions", "shopping.orders", "lodging.sessions", "lodging.bookings", "payments.obligations", "commerce_internal.replays"} {
		var n int
		must(t, x.f.app.QueryRow(`SELECT count(*) FROM `+table).Scan(&n))
		if n != 0 {
			t.Fatalf("unscoped runtime read %s", table)
		}
	}
}

func TestInternalGeneratedClientAndInvalidTransport(t *testing.T) {
	x := internalSetup(t)
	x.request(t, "PUT", "/shopping/catalog", x.shopManage, uuid.NewString(), shopCatalog, 200)
	client, err := ucpcontract.NewClient(x.server.URL, ucpcontract.WithRequestEditorFn(func(ctx context.Context, r *http.Request) error {
		r.Header.Set("Authorization", "Bearer "+x.shop)
		return nil
	}))
	must(t, err)
	res, err := client.CreateInternalCheckoutWithBody(context.Background(), &ucpcontract.CreateInternalCheckoutParams{IdempotencyKey: uuid.New(), RequestId: uuid.New(), UCPAgent: `profile="` + internalProfile + `"`}, "application/json", strings.NewReader(shoppingSelection))
	must(t, err)
	defer res.Body.Close()
	if res.StatusCode != 201 {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("generated client %d %s", res.StatusCode, raw)
	}
	for _, change := range []func(*http.Request){
		func(r *http.Request) { r.Header.Del("Request-Id") },
		func(r *http.Request) { r.Header.Del("Idempotency-Key") },
		func(r *http.Request) { r.Header.Set("UCP-Agent", `profile="https://other.example/.well-known/ucp"`) },
		func(r *http.Request) { r.Header.Set("Signature", "unselected") },
	} {
		r, err := http.NewRequest("POST", x.server.URL+"/shopping/checkout-sessions", strings.NewReader(shoppingSelection))
		must(t, err)
		r.Header.Set("Authorization", "Bearer "+x.shop)
		r.Header.Set("Request-Id", uuid.NewString())
		r.Header.Set("Idempotency-Key", uuid.NewString())
		r.Header.Set("UCP-Agent", `profile="`+internalProfile+`"`)
		r.Header.Set("Content-Type", "application/json")
		change(r)
		response, err := x.server.Client().Do(r)
		must(t, err)
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatalf("invalid transport admitted %d", response.StatusCode)
		}
	}
}
func TestInternalShoppingConcurrentStockAndStalePrice(t *testing.T) {
	x := internalSetup(t)
	x.request(t, "PUT", "/shopping/catalog", x.shopManage, uuid.NewString(), shopCatalog, 200)
	a := sessionID(t, x.request(t, "POST", "/shopping/checkout-sessions", x.shop, uuid.NewString(), shoppingSelection, 201))
	b := sessionID(t, x.request(t, "POST", "/shopping/checkout-sessions", x.shopOther, uuid.NewString(), shoppingSelection, 201))
	outcomes := make(chan string, 2)
	var wg sync.WaitGroup
	for _, pair := range [][2]string{{a, x.shop}, {b, x.shopOther}} {
		wg.Add(1)
		go func(id, token string) {
			defer wg.Done()
			outcomes <- sessionStatus(t, x.request(t, "POST", "/shopping/checkout-sessions/"+id+"/complete", token, uuid.NewString(), payLater, 200))
		}(pair[0], pair[1])
	}
	wg.Wait()
	close(outcomes)
	counts := map[string]int{}
	for status := range outcomes {
		counts[status]++
	}
	if counts["completed"] != 1 || counts["incomplete"] != 1 {
		t.Fatalf("stock race %+v", counts)
	}
	selectRed := strings.Replace(shoppingSelection, "shirt-blue", "shirt-red", 1)
	id := sessionID(t, x.request(t, "POST", "/shopping/checkout-sessions", x.shop, uuid.NewString(), selectRed, 201))
	updated := strings.Replace(strings.Replace(shopCatalog, `"base_revision":0`, `"base_revision":1`, 1), `"price_minor":2600`, `"price_minor":3000`, 1)
	x.request(t, "PUT", "/shopping/catalog", x.shopManage, uuid.NewString(), updated, 200)
	path := "/shopping/checkout-sessions/" + id
	if sessionStatus(t, x.request(t, "POST", path+"/complete", x.shop, uuid.NewString(), payLater, 200)) != "incomplete" {
		t.Fatal("silent repricing")
	}
	x.request(t, "PUT", path, x.shop, uuid.NewString(), selectRed, 200)
	if sessionStatus(t, x.request(t, "POST", path+"/complete", x.shop, uuid.NewString(), payLater, 200)) != "completed" {
		t.Fatal("reviewed updated price not usable")
	}
}
func hotelSelection(start, end string) string {
	return `{"property":{"id":"hotel"},"stays":[{"stay_dates":{"start_date":"` + start + `","end_date":"` + end + `"},"accommodation_type":{"id":"room-1"},"rate_plan":{"id":"flex"},"occupancy":{"total":2,"adults":2}}],"booker":{"first_name":"Example","last_name":"Guest","email":"guest@example.test"}}`
}
func TestInternalLodgingJourneyAndOverlap(t *testing.T) {
	x := internalSetup(t)
	x.request(t, "PUT", "/lodging/catalog", x.hotelManage, uuid.NewString(), hotelCatalog, 200)
	req := hotelSelection("2027-06-01", "2027-06-03")
	first := x.request(t, "POST", "/lodging/booking-sessions", x.hotel, uuid.NewString(), req, 201)
	a := sessionID(t, first)
	b := sessionID(t, x.request(t, "POST", "/lodging/booking-sessions", x.hotel, uuid.NewString(), req, 201))
	outcomes := make(chan string, 2)
	var wg sync.WaitGroup
	for _, id := range []string{a, b} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			outcomes <- sessionStatus(t, x.request(t, "POST", "/lodging/booking-sessions/"+id+"/complete", x.hotel, uuid.NewString(), payLater, 200))
		}(id)
	}
	wg.Wait()
	close(outcomes)
	counts := map[string]int{}
	for status := range outcomes {
		counts[status]++
	}
	if counts["completed"] != 1 || counts["incomplete"] != 1 {
		t.Fatalf("overlap race %+v", counts)
	}
	adjacent := sessionID(t, x.request(t, "POST", "/lodging/booking-sessions", x.hotel, uuid.NewString(), hotelSelection("2027-06-03", "2027-06-04"), 201))
	if sessionStatus(t, x.request(t, "POST", "/lodging/booking-sessions/"+adjacent+"/complete", x.hotel, uuid.NewString(), payLater, 200)) != "completed" {
		t.Fatal("adjacent stays overlap")
	}
	canceled := sessionID(t, x.request(t, "POST", "/lodging/booking-sessions", x.hotel, uuid.NewString(), hotelSelection("2027-07-01", "2027-07-02"), 201))
	if sessionStatus(t, x.request(t, "POST", "/lodging/booking-sessions/"+canceled+"/cancel", x.hotel, uuid.NewString(), `{}`, 200)) != "canceled" {
		t.Fatal("cancel failed")
	}
	var bookings, allocations, obligations int
	must(t, x.f.admin.QueryRow(`SELECT count(*) FROM lodging.bookings`).Scan(&bookings))
	must(t, x.f.admin.QueryRow(`SELECT count(*) FROM lodging.allocations`).Scan(&allocations))
	must(t, x.f.admin.QueryRow(`SELECT count(*) FROM payments.obligations WHERE domain='lodging' AND state='outstanding'`).Scan(&obligations))
	if bookings != 2 || allocations != 2 || obligations != 2 {
		t.Fatalf("unexpected completed records: %d/%d/%d", bookings, allocations, obligations)
	}
	// Bypass the application advisory locks deliberately: the SQL exclusion is
	// an independent final guard, not just a declared but unexercised constraint.
	_, err := x.f.admin.Exec(`INSERT INTO lodging.allocations(merchant_id,store_id,unit_id,booking_id,check_in,check_out) SELECT merchant_id,store_id,unit_id,booking_id,check_in+1,check_out+1 FROM lodging.allocations ORDER BY check_in LIMIT 1`)
	var sqlError interface{ SQLState() string }
	if !errors.As(err, &sqlError) || sqlError.SQLState() != "23P01" {
		t.Fatalf("expected overlap exclusion SQLSTATE23P01, got %v", err)
	}
}
func TestInternalDomainsNotExposedByPublicListener(t *testing.T) {
	x := internalSetup(t)
	for _, path := range []string{"/.well-known/ucp", "/shopping/checkout-sessions", "/lodging/booking-sessions", "/shopping/catalog", "/lodging/catalog"} {
		req, err := http.NewRequest("GET", x.f.server.URL+path, nil)
		must(t, err)
		req.Header.Set("Authorization", "Bearer "+x.shop)
		res, err := x.f.server.Client().Do(req)
		must(t, err)
		res.Body.Close()
		if res.StatusCode != 404 {
			t.Fatalf("public route exposed %s: %d", path, res.StatusCode)
		}
	}
	h, err := ucpinternal.NewHandler(x.f.app, ucpinternal.Config{Domains: "lodging", Endpoint: "https://internal.ocplab.org", Concurrency: 1})
	must(t, err)
	server := httptest.NewServer(h)
	defer server.Close()
	old := x.server
	x.server = server
	x.request(t, "POST", "/shopping/checkout-sessions", x.shop, uuid.NewString(), shoppingSelection, 404)
	x.server = old
}

func TestInternalSessionExpiryAndLodgingValidation(t *testing.T) {
	x := internalSetup(t)
	x.request(t, "PUT", "/shopping/catalog", x.shopManage, uuid.NewString(), shopCatalog, 200)
	x.request(t, "PUT", "/lodging/catalog", x.hotelManage, uuid.NewString(), hotelCatalog, 200)
	for _, invalid := range []string{
		hotelSelection("2027-06-03", "2027-06-01"),
		strings.Replace(hotelSelection("2027-06-01", "2027-06-03"), `"total":2`, `"total":3`, 1),
		strings.Replace(hotelSelection("2027-06-01", "2027-06-03"), `"room-1"`, `"unknown-room"`, 1),
	} {
		x.request(t, "POST", "/lodging/booking-sessions", x.hotel, uuid.NewString(), invalid, 400)
	}
	var missingBooker map[string]any
	must(t, json.Unmarshal([]byte(hotelSelection("2027-06-01", "2027-06-03")), &missingBooker))
	delete(missingBooker, "booker")
	initial := x.request(t, "POST", "/lodging/booking-sessions", x.hotel, uuid.NewString(), string(sessionJSON(t, missingBooker)), 201)
	if sessionStatus(t, initial) != "incomplete" {
		t.Fatal("booking without booker became ready")
	}
	path := "/lodging/booking-sessions/" + sessionID(t, initial)
	x.request(t, "GET", path, x.hotel, "", "", 200)
	x.request(t, "PUT", path, x.hotel, uuid.NewString(), hotelSelection("2027-06-01", "2027-06-03"), 200)
	shopping := sessionID(t, x.request(t, "POST", "/shopping/checkout-sessions", x.shop, uuid.NewString(), shoppingSelection, 201))
	for _, pair := range [][4]string{{"shopping", shopping, x.shop, "checkout-sessions"}, {"lodging", sessionID(t, initial), x.hotel, "booking-sessions"}} {
		_, err := x.f.admin.Exec(`UPDATE `+pair[0]+`.sessions SET body=jsonb_set(body,'{expires_at}',to_jsonb('2000-01-01T00:00:00Z'::text)) WHERE id=$1`, pair[1])
		must(t, err)
		body := x.request(t, "POST", "/"+pair[0]+"/"+pair[3]+"/"+pair[1]+"/complete", pair[2], uuid.NewString(), payLater, 200)
		if !bytes.Contains(body, []byte("session_expired")) {
			t.Fatal("expired session completed")
		}
	}
	var completed int
	must(t, x.f.admin.QueryRow(`SELECT (SELECT count(*) FROM shopping.orders)+(SELECT count(*) FROM lodging.bookings)+(SELECT count(*) FROM payments.obligations)`).Scan(&completed))
	if completed != 0 {
		t.Fatal("expired sessions allocated resources or obligations")
	}
}
