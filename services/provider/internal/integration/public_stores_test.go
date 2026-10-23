package integration

import (
	"context"
	"encoding/json"
	"ocp.local/platform/internal/database"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
)

func TestPublicStoreDirectory(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	byHandle := "/api/v1/public/stores/" + store.Handle
	f.request(t, "GET", byHandle, "", "", "", 404)
	const endpoint = "/api/v1/public/stores"
	read := func(path string) persistence.PublicStorePage {
		t.Helper()
		var p persistence.PublicStorePage
		body := f.request(t, "GET", path, "", "", "", 200)
		must(t, json.Unmarshal(body, &p))
		for _, field := range []string{"public_document", "offering_revision", "token_hash", "created_at", "scopes"} {
			if strings.Contains(string(body), `"`+field+`"`) {
				t.Fatalf("private field %s leaked", field)
			}
		}
		return p
	}
	if len(read(endpoint).Items) != 0 {
		t.Fatal("draft store listed")
	}
	private := f.merchantPath + "/" + store.ID.String()
	f.request(t, "PUT", private+"/publication", f.token, "directory-publish-001", `{"expected_generation":0,"revision":1}`, 201)
	var resolved persistence.PublicStore
	must(t, json.Unmarshal(f.request(t, "GET", byHandle, "", "", "", 200), &resolved))
	if resolved.ID != store.ID || resolved.MerchantID != store.MerchantID {
		t.Fatal("handle resolved a different store")
	}
	f.request(t, "GET", "/api/v1/public/stores/store-999999999999999999", "", "", "", 404)
	f.request(t, "GET", "/api/v1/public/stores/store-01", "", "", "", 400)
	page := read(endpoint + "?q=d%C3%96ner")
	if len(page.Items) != 1 || page.Items[0].ID != store.ID {
		t.Fatalf("name search failed: %+v", page)
	}
	if len(read(endpoint+"?q=%25").Items) != 0 {
		t.Fatal("search treated percent as wildcard")
	}
	// A published store on another merchant is visible through the narrow projection,
	// while the runtime role still cannot read arbitrary cross-tenant store rows.
	other := uuid.New()
	otherStore := uuid.New()
	_, err := f.admin.Exec(`INSERT INTO auth.merchants(id,display_name) VALUES($1,'Other merchant')`, other)
	must(t, err)
	_, err = f.admin.Exec(`INSERT INTO commerce.stores(merchant_id,id,display_name) VALUES($1,$2,'Other public store')`, other, otherStore)
	must(t, err)
	_, err = f.admin.Exec(`INSERT INTO commerce.offering_revisions(merchant_id,store_id,revision,model_id,model_version,document) VALUES($1,$2,1,'ocp.food.menu','1.0.0','{}')`, other, otherStore)
	must(t, err)
	_, err = f.admin.Exec(`UPDATE commerce.stores SET published_revision=1,public_document='{}' WHERE merchant_id=$1 AND id=$2`, other, otherStore)
	must(t, err)
	var otherNumber int64
	must(t, f.admin.QueryRow(`SELECT display_number FROM commerce.stores WHERE id=$1`, otherStore).Scan(&otherNumber))
	otherHandle := "/api/v1/public/stores/store-" + strconv.FormatInt(otherNumber, 10)
	f.request(t, "GET", otherHandle, "", "", "", 200)
	first := read(endpoint + "?limit=1")
	if len(first.Items) != 1 || first.NextAfter == nil {
		t.Fatal("missing directory continuation")
	}
	f.request(t, "GET", endpoint+"?q=Other&after="+*first.NextAfter, "", "", "", 400)
	second := read(endpoint + "?limit=1&after=" + *first.NextAfter)
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID || second.NextAfter != nil {
		t.Fatal("invalid directory continuation")
	}
	tx, err := database.TenantTransaction(context.Background(), f.app, store.MerchantID)
	must(t, err)
	var count int
	must(t, tx.QueryRow(`SELECT count(*) FROM commerce.stores WHERE merchant_id=$1`, other).Scan(&count))
	must(t, tx.Rollback())
	if count != 0 {
		t.Fatal("directory weakened tenant RLS")
	}
	for _, function := range []string{"commerce.list_public_stores(text,uuid,uuid,integer)", "commerce.public_store_by_number(bigint)"} {
		var definer, runtimeExecute, publicExecute bool
		var owner, config string
		must(t, f.admin.QueryRow(`SELECT p.prosecdef,r.rolname,array_to_string(p.proconfig,','),
 has_function_privilege('ocp_app',p.oid,'EXECUTE'),
 EXISTS(SELECT FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')
 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner
 WHERE p.oid=$1::regprocedure`, function).Scan(&definer, &owner, &config, &runtimeExecute, &publicExecute))
		if !definer || owner == "ocp_app" || config != "search_path=pg_catalog" || !runtimeExecute || publicExecute {
			t.Fatal("public projection function authority changed")
		}
	}
	_, err = f.admin.Exec(`UPDATE commerce.stores SET publication_suspended=true WHERE merchant_id=$1 AND id=$2`, other, otherStore)
	must(t, err)
	f.request(t, "GET", otherHandle, "", "", "", 404)
	if len(read(endpoint).Items) != 1 {
		t.Fatal("suspended store listed")
	}
	f.request(t, "PUT", private+"/publication", f.token, "directory-withdraw-001", `{"expected_generation":1}`, 201)
	f.request(t, "GET", byHandle, "", "", "", 404)
	if len(read(endpoint).Items) != 0 {
		t.Fatal("withdrawn store listed")
	}
	f.request(t, "GET", endpoint+"?after=bad", "", "", "", 400)
	f.request(t, "GET", endpoint+"?limit=101", "", "", "", 400)
	f.request(t, "GET", endpoint+"?q="+strings.Repeat("a", 161), "", "", "", 400)
}
