package integration

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/orderupdates"
	"ocp.local/platform/internal/persistence"
)

func TestPublicOfferingStreamsIsolateDraftsOrdersAndOtherStores(t *testing.T) {
	f := setup(t)
	var database string
	must(t, f.admin.QueryRow("SELECT current_database()").Scan(&database))
	dsn, err := url.Parse(os.Getenv("OCP_TEST_DATABASE_URL"))
	must(t, err)
	dsn.Path = "/" + database
	dsn.User = url.UserPassword("ocp_app", "ocp_app_local_only")
	ctx, cancel := context.WithCancel(context.Background())
	hub := orderupdates.NewOfferings()
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
		t.Fatal("public listener not ready")
	}
	handler, err := httpapi.NewHandler(f.app, httpapi.WithOfferingUpdates(hub))
	must(t, err)
	original := f.server
	f.server = httptest.NewServer(handler)
	defer func() { f.server.Close(); f.server = original }()
	store := preparedStore(t, f)
	private := f.merchantPath + "/" + store.ID.String()
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	f.request(t, "GET", public+"/offering-updates", "", "", "", 404)
	f.request(t, "PUT", private+"/publication", f.token, "publish-public-stream-001", `{"expected_generation":0,"revision":1}`, 201)
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(f.server.URL + public + "/offering-updates")
	must(t, err)
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("public SSE failed")
	}
	reader := bufio.NewReader(response.Body)
	frame := func() string {
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
	if got := frame(); !strings.Contains(got, `"resource":"public_offering"`) || strings.Contains(got, store.ID.String()) {
		t.Fatalf("bad public frame: %s", got)
	}
	signal, release, ok := hub.Subscribe(orderupdates.Change{Merchant: f.principal.MerchantID, Store: store.ID}, uuid.New())
	if !ok {
		t.Fatal("subscribe")
	}
	defer release()
	quiet := func() {
		t.Helper()
		select {
		case <-signal:
			t.Fatal("private, foreign, or rolled-back change leaked")
		case <-time.After(100 * time.Millisecond):
		}
	}
	// Draft-only store writes have no public meaning and must not emit a hint.
	tx, err := persistence.TenantTransaction(ctx, f.app, f.principal.MerchantID)
	must(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE commerce.stores SET display_name=display_name WHERE merchant_id=$1 AND id=$2", f.principal.MerchantID, store.ID)
	must(t, err)
	must(t, tx.Commit())
	quiet()
	tx, err = persistence.TenantTransaction(ctx, f.app, f.principal.MerchantID)
	must(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE commerce.stores SET ordering_revision=ordering_revision+1 WHERE merchant_id=$1 AND id=$2", f.principal.MerchantID, store.ID)
	must(t, err)
	must(t, tx.Rollback())
	quiet()
	// Even a forged order-channel payload cannot enter this separate listener.
	_, err = f.admin.Exec("SELECT pg_notify('ocp_food_order_changed', $1)", `{"Merchant":"`+f.principal.MerchantID.String()+`","Store":"`+store.ID.String()+`","Order":"`+uuid.NewString()+`"}`)
	must(t, err)
	quiet()
	_, err = f.admin.Exec("SELECT pg_notify('ocp_public_offering_changed', $1)", `{"Merchant":"`+f.principal.MerchantID.String()+`","Store":"`+uuid.NewString()+`"}`)
	must(t, err)
	quiet()
	f.request(t, "PUT", private+"/food/ordering", f.token, "pause-public-stream-001", `{"expected_revision":0,"accepting_orders":false,"acceptance_mode":"manual"}`, 201)
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("committed availability change missing")
	}
	if got := frame(); !strings.Contains(got, "event: invalidate") {
		t.Fatal("missing availability SSE")
	}
	f.request(t, "PUT", private+"/publication", f.token, "withdraw-public-stream-001", `{"expected_generation":1,"revision":null}`, 201)
	if got := frame(); !strings.Contains(got, "event: invalidate") {
		t.Fatal("withdrawal not signaled")
	}
	f.request(t, "GET", public+"/offering-updates", "", "", "", 404)
}
