package integration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/google/uuid"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/discovery"
	"ocp.local/platform/internal/infra/database"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func startRole(t *testing.T, f *fixture, role, domains string, extra ...string) (string, func()) {
	t.Helper()
	binary := os.Getenv("OCP_TEST_API_BINARY")
	if binary == "" {
		t.Skip("requires task test:domains (compiled runtime processes)")
	}
	var dbName string
	must(t, f.admin.QueryRow(`SELECT current_database()`).Scan(&dbName))
	dsn, err := url.Parse(os.Getenv("OCP_TEST_DATABASE_URL"))
	must(t, err)
	dsn.Path = "/" + dbName
	dsn.User = url.UserPassword("ocp_app", "ocp_app_local_only")
	query := dsn.Query()
	query.Set("application_name", "role-"+domains)
	dsn.RawQuery = query.Encode()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	must(t, err)
	address := listener.Addr().String()
	must(t, listener.Close())
	selector := "-role"
	if role == "core" || role == "food" || role == "shopping" || role == "lodging" {
		selector = "-service"
	}
	cmd := exec.Command(binary, selector, role, "-listen", address)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "OCP_DATABASE_URL=" + dsn.String(), "OCP_ENABLED_DOMAINS=" + domains, "OCP_INTERNAL_ENDPOINT=https://internal.ocplab.org", "OCP_DOMAIN_CONCURRENCY=1", "OCP_DB_MAX_OPEN=2", "OCP_DB_MAX_IDLE=1", "OCP_OTEL_EXPORTER=none"}
	cmd.Env = append(cmd.Env, extra...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	must(t, cmd.Start())
	var once sync.Once
	stop := func() { once.Do(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }) }
	t.Cleanup(stop)
	base := "http://" + address
	client := http.Client{Timeout: 250 * time.Millisecond}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		r, err := client.Get(base + "/health/live")
		if err == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				return base, stop
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("runtime process failed readiness")
	return "", stop
}

func TestDomainProcessIsolationRecoveryAndSaturation(t *testing.T) {
	if os.Getenv("OCP_TEST_API_BINARY") == "" {
		t.Skip("requires task test:domains")
	}
	x := internalSetup(t)
	shop, stopShop := startRole(t, x.f, "shopping", "shopping")
	shopReplica, _ := startRole(t, x.f, "shopping", "shopping")
	hotel, _ := startRole(t, x.f, "lodging", "lodging")
	food, _ := startRole(t, x.f, "public", "food")
	x.apiURL = shop
	x.request(t, "PUT", "/shopping/catalog", x.shopManage, uuid.NewString(), shopCatalog, 200)
	key := uuid.NewString()
	first := x.request(t, "POST", "/shopping/checkout-sessions", x.shop, key, shoppingSelection, 201)
	stopShop()
	x.apiURL = shopReplica
	replay := x.request(t, "POST", "/shopping/checkout-sessions", x.shop, key, shoppingSelection, 201)
	if string(first) != string(replay) {
		t.Fatal("process restart/replica replay changed")
	}
	x.apiURL = hotel
	x.request(t, "PUT", "/lodging/catalog", x.hotelManage, uuid.NewString(), hotelCatalog, 200)
	x.request(t, "GET", "/shopping/checkout-sessions/"+sessionID(t, first), x.shop, "", "", 404)
	x.apiURL = food
	response, err := http.Get(food + "/lodging/booking-sessions")
	must(t, err)
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal("public process exposed Lodging")
	}
	// Hold the Shopping catalog on the shared PostgreSQL cluster. Its sole
	// application slot becomes busy; another Shopping call is bounded at 429,
	// while Lodging can still price a stay through its separate process/pool.
	tx, err := x.f.admin.BeginTx(context.Background(), nil)
	must(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`SELECT 1 FROM shopping.catalogs FOR UPDATE`)
	must(t, err)
	started := make(chan struct{})
	done := make(chan int, 1)
	go func() {
		r, _ := http.NewRequest("POST", shopReplica+"/shopping/checkout-sessions", strings.NewReader(shoppingSelection))
		r.Header.Set("Authorization", "Bearer "+x.shop)
		r.Header.Set("Request-Id", uuid.NewString())
		r.Header.Set("Idempotency-Key", uuid.NewString())
		r.Header.Set("UCP-Agent", `profile="`+internalProfile+`"`)
		r.Header.Set("Content-Type", "application/json")
		close(started)
		res, e := http.DefaultClient.Do(r)
		if e != nil {
			done <- 0
			return
		}
		res.Body.Close()
		done <- res.StatusCode
	}()
	<-started
	deadline := time.Now().Add(2 * time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		must(t, x.f.admin.QueryRow(`SELECT EXISTS(SELECT FROM pg_stat_activity WHERE application_name='role-shopping' AND wait_event_type='Lock')`).Scan(&waiting))
		if waiting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("Shopping contention was not observed")
	}
	x.apiURL = shopReplica
	x.request(t, "POST", "/shopping/checkout-sessions", x.shop, uuid.NewString(), shoppingSelection, 429)
	x.apiURL = hotel
	start := time.Now()
	x.request(t, "POST", "/lodging/booking-sessions", x.hotel, uuid.NewString(), hotelSelection("2027-08-01", "2027-08-02"), 201)
	if time.Since(start) > 4*time.Second {
		t.Fatal("Lodging exhausted its independent process budget")
	}
	must(t, tx.Rollback())
	if <-done != 201 {
		t.Fatal("blocked Shopping operation failed after release")
	}
	var connections int
	must(t, x.f.admin.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE application_name='role-shopping'`).Scan(&connections))
	if connections > 2 {
		t.Fatalf("surviving Shopping process exceeded pool budget: %d", connections)
	}
	t.Log("Food-only, Shopping-only replicas, Lodging-only: route isolation, replay after actual process kill, bounded Shopping saturation, Lodging progress, two-connection pool ceiling passed; shared cluster remains a common failure/capacity boundary")
}

func TestWorkerProcessRecoveryAfterExternalReceipt(t *testing.T) {
	if os.Getenv("OCP_TEST_API_BINARY") == "" {
		t.Skip("requires task test:domains")
	}
	f := setup(t)
	store := preparedStore(t, f)
	ctx := context.Background()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	must(t, err)
	received := make(chan struct{}, 2)
	var mu sync.Mutex
	seen := map[string]int{}
	hold := true
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Message struct {
				Catalogs []networkcatalog.Catalog `json:"catalogs"`
			} `json:"message"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || len(request.Message.Catalogs) != 1 {
			w.WriteHeader(400)
			return
		}
		id := request.Message.Catalogs[0].ID
		mu.Lock()
		seen[id]++
		block := hold
		mu.Unlock()
		received <- struct{}{}
		if block {
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "COMPLETED", "results": []any{map[string]string{"catalogId": id, "status": "ACCEPTED"}}})
	}))
	defer endpoint.Close()
	binding := networkcatalog.Binding{ProviderID: "fixture.lab", MerchantID: store.MerchantID, StoreID: store.ID, APIBase: "http://fixture/api/v1", PublicKey: base64.StdEncoding.EncodeToString(public)}
	config := networkcatalog.PublisherConfig{Endpoint: endpoint.URL, Bindings: []networkcatalog.Binding{binding}, PrivateKey: base64.StdEncoding.EncodeToString(private)}
	path := filepath.Join(t.TempDir(), "publisher.json")
	must(t, os.WriteFile(path, sessionJSON(t, config), 0600))
	tx, err := database.TenantTransaction(ctx, f.app, store.MerchantID)
	must(t, err)
	id, err := persistence.EnqueueEffect(ctx, tx, persistence.Effect{MerchantID: store.MerchantID, StoreID: store.ID, Domain: capability.Food, Kind: "catalog.publish", SubjectID: store.ID, Key: "crash-receipt", Payload: catalogPayload(t)})
	must(t, err)
	must(t, tx.Commit())
	_, stop := startRole(t, f, "worker", "food", "OCP_LOCAL_NETWORK=true", "OCP_NETWORK_PUBLISHER_CONFIG="+path)
	select {
	case <-received:
	case <-time.After(12 * time.Second):
		t.Fatal("worker did not deliver")
	}
	stop()
	mu.Lock()
	hold = false
	mu.Unlock()
	_, err = f.admin.Exec(`UPDATE runtime.effects SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, id)
	must(t, err)
	_, _ = startRole(t, f, "worker", "food", "OCP_LOCAL_NETWORK=true", "OCP_NETWORK_PUBLISHER_CONFIG="+path)
	deadline := time.Now().Add(12 * time.Second)
	state := ""
	for time.Now().Before(deadline) {
		must(t, f.admin.QueryRow(`SELECT state FROM runtime.effects WHERE id=$1`, id).Scan(&state))
		if state == "completed" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if state != "completed" {
		t.Fatal("restarted worker did not finish recovered intent")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 {
		t.Fatal("retry changed external effect identity")
	}
	for _, calls := range seen {
		if calls != 2 {
			t.Fatalf("expected receipt then retry; got %d", calls)
		}
	}
	t.Log("Actual worker process killed after HTTP receiver retained catalog identity, before acknowledgement; restart replayed same immutable catalog ID and completed fenced intent. Receiver is a test double; separate test:network covers native ONIX.")
}
func sessionJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	must(t, err)
	return raw
}
