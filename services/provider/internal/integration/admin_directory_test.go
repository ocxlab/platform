package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/persistence"
)

func TestAdminStoreDirectory(t *testing.T) {
	f := setup(t)
	preparedStore(t, f)
	other := uuid.New()
	otherStore := uuid.New()
	_, err := f.admin.Exec(`INSERT INTO auth.merchants(id,display_name) VALUES($1,'Second merchant')`, other)
	must(t, err)
	_, err = f.admin.Exec(`INSERT INTO commerce.stores(merchant_id,id,display_name,publication_suspended) VALUES($1,$2,'Pizza workshop',true)`, other, otherStore)
	must(t, err)
	provider, token, err := persistence.ProvisionProvider(context.Background(), f.admin, []string{"publication:read"}, time.Now().Add(time.Hour))
	must(t, err)
	const path = "/api/v1/admin/stores"
	read := func(suffix string) persistence.AdminPage {
		t.Helper()
		var page persistence.AdminPage
		body := f.request(t, "GET", path+suffix, token, "", "", 200)
		must(t, json.Unmarshal(body, &page))
		for _, field := range []string{"token_hash", "public_document", "document", "guest_id", "scopes"} {
			if strings.Contains(string(body), `"`+field+`"`) {
				t.Fatal("private field exposed", field)
			}
		}
		return page
	}
	f.request(t, "GET", path, "", "", "", 401)
	f.request(t, "GET", path, f.token, "", "", 401)
	_, wrong, err := persistence.ProvisionProvider(context.Background(), f.admin, []string{"publication:manage"}, time.Now().Add(time.Hour))
	must(t, err)
	f.request(t, "GET", path, wrong, "", "", 403)
	first := read("?limit=1")
	if len(first.Items) != 1 || first.NextAfter == nil {
		t.Fatal("missing page")
	}
	var summary map[string]any
	must(t, json.Unmarshal(first.Items[0], &summary))
	if summary["id"] != otherStore.String() || summary["merchant_name"] != "Second merchant" {
		t.Fatal("wrong projection", summary)
	}
	second := read("?limit=1&after=" + *first.NextAfter)
	if len(second.Items) != 1 || second.NextAfter != nil {
		t.Fatal("missing continuation")
	}
	if len(read("?status=suspended&q=workshop").Items) != 1 || len(read("?q=Second").Items) != 1 || len(read("?q=%25").Items) != 0 {
		t.Fatal("invalid search/filter")
	}
	f.request(t, "GET", path+"?status=suspended&after="+*first.NextAfter, token, "", "", 400)
	f.request(t, "GET", path+"?after=bad", token, "", "", 400)
	f.request(t, "GET", path+"?limit=101", token, "", "", 400)
	f.request(t, "GET", path+"?q="+strings.Repeat("x", 161), token, "", "", 400)
	f.request(t, "GET", "/api/v1/admin/food/orders?after="+*first.NextAfter, token, "", "", 403)
	var count int
	must(t, f.app.QueryRow(`SELECT count(*) FROM commerce.stores`).Scan(&count))
	if count != 0 {
		t.Fatal("tenant RLS weakened")
	}
	for _, signature := range []string{"commerce.admin_stores(uuid,text,text,bigint,integer)", "commerce.admin_orders(uuid,text,text,bigint,integer)"} {
		var definer, runtimeExecute, publicExecute bool
		var owner, config string
		must(t, f.admin.QueryRow(`SELECT p.prosecdef,r.rolname,array_to_string(p.proconfig,','),
 has_function_privilege('ocp_app',p.oid,'EXECUTE'),
 EXISTS(SELECT FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')
 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, signature).Scan(&definer, &owner, &config, &runtimeExecute, &publicExecute))
		if !definer || owner == "ocp_app" || config != "search_path=pg_catalog" || !runtimeExecute || publicExecute {
			t.Fatal("unsafe admin projection authority", signature)
		}
	}
	_, err = f.admin.Exec(`UPDATE auth.provider_grants SET revoked=true WHERE id=$1`, provider.ID)
	must(t, err)
	f.request(t, "GET", path, token, "", "", 401)
	must(t, f.app.QueryRow(`SELECT count(*) FROM commerce.admin_stores($1,'','',NULL,25)`, provider.ID).Scan(&count))
	if count != 0 {
		t.Fatal("SQL projection accepted revoked grant")
	}
}

func TestAdminOrderDirectory(t *testing.T) {
	f, store, _, guest, guestToken, order := placedOrderFixture(t)
	_, token, err := persistence.ProvisionProvider(context.Background(), f.admin, []string{"order:inspect"}, time.Now().Add(time.Hour))
	must(t, err)
	const path = "/api/v1/admin/food/orders"
	f.request(t, "GET", path, "", "", "", 401)
	f.request(t, "GET", path, f.token, "", "", 401)
	f.request(t, "GET", path, guestToken, "", "", 401)
	var page persistence.AdminPage
	body := f.request(t, "GET", path+"?status=pending", token, "", "", 200)
	must(t, json.Unmarshal(body, &page))
	if len(page.Items) != 1 {
		t.Fatal("missing order")
	}
	var summary map[string]any
	must(t, json.Unmarshal(page.Items[0], &summary))
	if summary["id"] != order.ID.String() || summary["store_id"] != store.ID.String() || summary["reference"] != order.Reference || summary["payment"] != "not_started" {
		t.Fatal("wrong summary", summary)
	}
	for _, field := range []string{"terms", "guest_id", "token", "history", "amount_minor"} {
		if strings.Contains(string(body), `"`+field+`"`) {
			t.Fatal("list exposes detail", field)
		}
	}
	f.request(t, "GET", path+"?status=published", token, "", "", 400)
	f.request(t, "GET", "/api/v1/admin/stores", token, "", "", 403)
	f.request(t, "POST", guest+"/food/orders/"+order.ID.String()+"/cancellation", guestToken, "admin-directory-cancel-001", `{"expected_revision":1}`, 201)
	must(t, json.Unmarshal(f.request(t, "GET", path+"?status=cancelled", token, "", "", 200), &page))
	if len(page.Items) != 1 {
		t.Fatal("cancelled filter failed")
	}
	must(t, json.Unmarshal(f.request(t, "GET", path+"?status=pending", token, "", "", 200), &page))
	if len(page.Items) != 0 {
		t.Fatal("resolution not preferred over acceptance")
	}
	var quote persistence.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", guest+"/food/quotes", guestToken, "", selection(1), 201), &quote))
	f.request(t, "POST", guest+"/food/orders", guestToken, "admin-directory-second-001", fmt.Sprintf(`{"quote_id":%q,"fulfilment":"pickup"}`, quote.ID), 201)
	must(t, json.Unmarshal(f.request(t, "GET", path+"?limit=1", token, "", "", 200), &page))
	if len(page.Items) != 1 || page.NextAfter == nil {
		t.Fatal("missing order continuation")
	}
	first := string(page.Items[0])
	cursor := *page.NextAfter
	page = persistence.AdminPage{}
	must(t, json.Unmarshal(f.request(t, "GET", path+"?limit=1&after="+cursor, token, "", "", 200), &page))
	if len(page.Items) != 1 || string(page.Items[0]) == first || page.NextAfter != nil {
		t.Fatal("invalid order continuation")
	}
	f.request(t, "GET", path+"?status=cancelled&after="+cursor, token, "", "", 400)
	f.request(t, "GET", path+"?q=changed&after="+cursor, token, "", "", 400)
	// Numeric padding must never truncate references once six digits are exceeded.
	_, err = f.admin.Exec(`ALTER TABLE commerce.food_orders ALTER COLUMN display_number RESTART WITH 1000001`)
	must(t, err)
	_, err = f.admin.Exec(`UPDATE commerce.food_orders SET display_number=DEFAULT WHERE id=$1`, order.ID)
	must(t, err)
	must(t, json.Unmarshal(f.request(t, "GET", path+"?q=O-1000001", token, "", "", 200), &page))
	if len(page.Items) != 1 || !strings.Contains(string(page.Items[0]), `"O-1000001"`) {
		t.Fatal("reference was truncated")
	}
}
