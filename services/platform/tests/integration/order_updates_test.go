package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	foodstore "ocp.local/platform/internal/food/persistence"
	"ocp.local/platform/internal/infra/database"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/cmd/api/transport/httpapi"
	"ocp.local/platform/internal/core/updates"
	"ocp.local/platform/internal/infra/telemetry"
)

func TestOrderStreamsUseCommittedScopedHintsAndOrdinaryReads(t *testing.T) {
	f := setup(t)
	var databaseName string
	must(t, f.admin.QueryRow("SELECT current_database()").Scan(&databaseName))
	dsn, err := url.Parse(os.Getenv("OCP_TEST_DATABASE_URL"))
	must(t, err)
	dsn.Path = "/" + databaseName
	dsn.User = url.UserPassword("ocp_app", "ocp_app_local_only")
	ctx, cancel := context.WithCancel(context.Background())
	hub := orderupdates.New()
	finished := make(chan struct{})
	go func() { defer close(finished); hub.Run(ctx, dsn.String()) }()
	t.Cleanup(func() { cancel(); <-finished })
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if _, release, ok := hub.Subscribe(orderupdates.Change{}, uuid.New()); ok {
			release()
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("listener did not become ready")
	}
	// Exercise real SSE flushing/deadlines through the production telemetry wrapper,
	// without requiring a collector or exporting any private fixture data.
	signals, err := telemetry.New(ctx, telemetry.Config{Exporter: "none", SampleRatio: 0}, nil)
	must(t, err)
	defer func() { must(t, signals.Shutdown(context.Background())) }()
	handler, err := httpapi.NewHandler(f.app, httpapi.WithOrderUpdates(hub), httpapi.WithTelemetry(signals.Observer))
	must(t, err)
	original := f.server
	f.server = httptest.NewServer(handler)
	defer func() { f.server.Close(); f.server = original }()
	store := preparedStore(t, f)
	private := f.merchantPath + "/" + store.ID.String()
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	guestPath := strings.Replace(private, "/api/v1/", "/api/v1/guest/", 1)
	f.request(t, "PUT", private+"/publication", f.token, "publish-stream-menu-001", `{"expected_generation":0,"revision":1}`, 201)
	var guest, stranger foodstore.GuestSession
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &guest))
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &stranger))
	var quote foodstore.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guestPath+"/food/quotes", guest.Token, "", selection(1), 201), &quote))
	var order foodstore.FoodOrder
	must(t, json.Unmarshal(f.request(t, "POST", guestPath+"/food/orders", guest.Token, "stream-placement-001", fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, quote.ID), 201), &order))
	updates := guestPath + "/food/order-updates/" + order.ID.String()
	f.request(t, "GET", updates, "", "", "", 401)
	f.request(t, "GET", updates, f.token, "", "", 401)
	f.request(t, "GET", updates, stranger.Token, "", "", 404)
	f.request(t, "GET", private+"/food/order-updates", guest.Token, "", "", 401)
	f.request(t, "GET", strings.Replace(updates, store.ID.String(), uuid.NewString(), 1), guest.Token, "", "", 403)
	open := func(path, token string) (*http.Response, *bufio.Reader) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, "GET", f.server.URL+path, nil)
		must(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		client := &http.Client{Timeout: 20 * time.Second}
		response, err := client.Do(req)
		must(t, err)
		if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" {
			response.Body.Close()
			t.Fatalf("bad stream response: %d", response.StatusCode)
		}
		return response, bufio.NewReader(response.Body)
	}
	frame := func(reader *bufio.Reader) string {
		t.Helper()
		var result strings.Builder
		for {
			line, err := reader.ReadString('\n')
			must(t, err)
			result.WriteString(line)
			if line == "\n" {
				return result.String()
			}
		}
	}
	response, reader := open(updates, guest.Token)
	defer response.Body.Close()
	if got := frame(reader); !strings.Contains(got, "event: invalidate") || strings.Contains(got, order.ID.String()) {
		t.Fatalf("bad initial frame: %s", got)
	}
	merchantResponse, merchantReader := open(private+"/food/order-updates", f.token)
	defer merchantResponse.Body.Close()
	_ = frame(merchantReader)
	signal, release, ok := hub.Subscribe(orderupdates.Change{Merchant: order.MerchantID, Store: order.StoreID, Order: order.ID}, uuid.New())
	if !ok {
		t.Fatal("subscribe")
	}
	defer release()
	// A rolled-back event must not escape as a hint.
	tx, err := database.TenantTransaction(ctx, f.app, f.principal.MerchantID)
	must(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO commerce.food_order_events(merchant_id,store_id,order_id,sequence,event,actor_id,request_id) VALUES($1,$2,$3,2,'accepted',$4,$5)`, order.MerchantID, order.StoreID, order.ID, f.principal.ID, uuid.New())
	must(t, err)
	must(t, tx.Rollback())
	select {
	case <-signal:
		t.Fatal("rollback emitted hint")
	case <-time.After(100 * time.Millisecond):
	}
	f.request(t, "POST", private+"/food/orders/"+order.ID.String()+"/decisions", f.token, "stream-accept-order-001", `{"expected_revision":1,"decision":"accept"}`, 201)
	if got := frame(reader); !strings.Contains(got, "event: invalidate") {
		t.Fatalf("no guest hint: %s", got)
	}
	if got := frame(merchantReader); !strings.Contains(got, "event: invalidate") {
		t.Fatalf("no merchant hint: %s", got)
	}
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("committed hint missing")
	}
	var current foodstore.FoodOrder
	must(t, json.Unmarshal(f.request(t, "GET", guestPath+"/food/orders/"+order.ID.String(), guest.Token, "", "", 200), &current))
	if current.Acceptance != "accepted" || current.Revision != 2 || len(current.History) != 2 {
		t.Fatalf("authoritative state missing: %+v", current)
	}
	// Reconnect does not require a cursor: even an invalid/missing prior event ID
	// cannot prevent the initial fresh-read hint.
	again, againReader := open(updates, guest.Token)
	if !strings.Contains(frame(againReader), "invalidate") {
		t.Fatal("no reconnect resync")
	}
	again.Body.Close()
	// A live grant revocation terminates the existing stream at its next heartbeat.
	_, err = f.admin.Exec(`UPDATE commerce.guest_grants SET revoked=true WHERE id=(SELECT guest_id FROM commerce.food_orders WHERE id=$1)`, order.ID)
	must(t, err)
	if line, err := reader.ReadString('\n'); err == nil {
		t.Fatalf("revoked stream continued: %s", line)
	}
	f.request(t, "GET", updates, guest.Token, "", "", 401)
}
