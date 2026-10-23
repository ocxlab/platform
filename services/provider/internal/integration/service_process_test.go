package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	foodstore "ocp.local/platform/internal/food/persistence"
)

// Real processes share PostgreSQL, not an in-process HTTP server or copied state.
func TestServiceProcessCoreFoodAndReplicaReplay(t *testing.T) {
	if os.Getenv("OCP_TEST_API_BINARY") == "" {
		t.Skip("requires task test:domains")
	}
	f := setup(t)
	core, stopCore := startRole(t, f, "core", "food")
	food, _ := startRole(t, f, "food", "food", "OCP_DEVELOPMENT_PAYMENTS=true")
	f.server.URL = core
	store := preparedStore(t, f)
	private := f.merchantPath + "/" + store.ID.String()
	public := strings.Replace(private, "/api/v1/", "/api/v1/public/", 1)
	guest := strings.Replace(private, "/api/v1/", "/api/v1/guest/", 1)
	f.request(t, "PUT", private+"/publication", f.token, "split-publish-menu-001", `{"expected_generation":0,"revision":1}`, 201)
	f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 503)
	f.server.URL = food
	f.request(t, "GET", private, f.token, "", "", 503)
	var session foodstore.GuestSession
	must(t, json.Unmarshal(f.request(t, "POST", public+"/guest-sessions", "", "", `{}`, 201), &session))
	var quote foodstore.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", session.Token, "", selection(1), 201), &quote))
	path := guest + "/food/orders"
	body := fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, quote.ID)
	key := "split-place-order-001"
	first := f.request(t, "POST", path, session.Token, key, body, 201)
	var order foodstore.FoodOrder
	must(t, json.Unmarshal(first, &order))
	stopCore()
	// Taking core offline does not block Food order reads or merchant decisions.
	f.request(t, "GET", path+"/"+order.ID.String(), session.Token, "", "", 200)
	f.request(t, "POST", private+"/food/orders/"+order.ID.String()+"/decisions", f.token, "split-accept-order-001", `{"expected_revision":1,"decision":"accept"}`, 201)
	replica, _ := startRole(t, f, "food", "food", "OCP_DEVELOPMENT_PAYMENTS=true")
	f.server.URL = replica
	if !bytes.Equal(first, f.request(t, "POST", path, session.Token, key, body, 201)) {
		t.Fatal("split Food replica changed the placement replay")
	}
	core, _ = startRole(t, f, "core", "food")
	f.server.URL = core
	f.request(t, "GET", private, f.token, "", "", 200)
}
