package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/httpapi"
)

func TestManagedStoreList(t *testing.T) {
	f := setup(t)
	readPage := func(path, token string) persistence.StorePage {
		t.Helper()
		var page persistence.StorePage
		must(t, json.Unmarshal(f.request(t, "GET", path, token, "", "", 200), &page))
		return page
	}
	page := readPage(f.merchantPath, f.token)
	if page.Items == nil || len(page.Items) != 0 || page.NextAfter != nil {
		t.Fatalf("empty merchant must return an empty array: %+v", page)
	}
	wanted := map[uuid.UUID]bool{}
	for i := 0; i < 2; i++ {
		var store persistence.Store
		body := fmt.Sprintf(`{"display_name":"Kitchen %d"}`, i+1)
		must(t, json.Unmarshal(f.request(t, "POST", f.merchantPath, f.token, fmt.Sprintf("managed-store-%02d", i), body, 201), &store))
		wanted[store.ID] = true
	}
	other, otherToken, err := persistence.Provision(context.Background(), f.admin, "Other merchant", []string{"store:read", "store:write"}, time.Now().Add(time.Hour))
	must(t, err)
	otherPath := "/api/v1/merchants/" + other.MerchantID.String() + "/stores"
	var foreign persistence.Store
	must(t, json.Unmarshal(f.request(t, "POST", otherPath, otherToken, "other-store-create-001", `{"display_name":"Other merchant kitchen"}`, 201), &foreign))

	page = readPage(f.merchantPath, f.token)
	if len(page.Items) != 2 || page.NextAfter != nil {
		t.Fatalf("default page: %+v", page)
	}
	selected := page.Items[0]
	filtered := readPage(f.merchantPath+"?handle="+selected.Handle, f.token)
	if len(filtered.Items) != 1 || filtered.Items[0].ID != selected.ID || filtered.NextAfter != nil {
		t.Fatal("exact handle lookup must return the selected store")
	}
	filtered = readPage(f.merchantPath+"?handle="+foreign.Handle, f.token)
	if len(filtered.Items) != 0 {
		t.Fatal("handle lookup exposed another merchant's store")
	}
	filtered = readPage(f.merchantPath+"?handle=store-999999999999999999", f.token)
	if len(filtered.Items) != 0 {
		t.Fatal("unknown handle matched a store")
	}
	seen := map[uuid.UUID]bool{}
	path := f.merchantPath + "?limit=1"
	var previous string
	for i := 0; i < 2; i++ {
		page = readPage(path, f.token)
		if len(page.Items) != 1 {
			t.Fatalf("page %d: %+v", i, page)
		}
		store := page.Items[0]
		if store.DraftRevision != 0 || store.MenuItemCount == nil || *store.MenuItemCount != 0 || !store.UpdatedAt.Equal(store.CreatedAt) {
			t.Fatalf("new store summary: %+v", store)
		}
		if !wanted[store.ID] || seen[store.ID] || store.MerchantID != f.principal.MerchantID || store.ID.String() <= previous {
			t.Fatal("store pagination leaked, duplicated, or reordered records")
		}
		previous = store.ID.String()
		seen[store.ID] = true
		if i == 0 {
			if page.NextAfter == nil || *page.NextAfter != store.ID {
				t.Fatal("missing next-page position")
			}
			path = f.merchantPath + "?limit=1&after=" + page.NextAfter.String()
		} else if page.NextAfter != nil {
			t.Fatal("last page incorrectly advertises continuation")
		}
	}
	page = readPage(f.merchantPath+"?after="+previous, f.token)
	if len(page.Items) != 0 {
		t.Fatal("exclusive cursor repeated the last store")
	}
	page = readPage(otherPath, otherToken)
	if len(page.Items) != 1 || page.Items[0].ID != foreign.ID {
		t.Fatal("other merchant cannot see its own single store")
	}
	// Even a known foreign cursor is only a position within the authorized tenant.
	page = readPage(f.merchantPath+"?after="+foreign.ID.String(), f.token)
	for _, store := range page.Items {
		if !wanted[store.ID] {
			t.Fatal("foreign cursor exposed another tenant")
		}
	}
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=wrong", "?after=not-a-uuid", "?handle=store-01", "?handle=other", "?handle=store-9999999999999999999"} {
		f.request(t, "GET", f.merchantPath+query, f.token, "", "", 400)
	}
	f.request(t, "GET", f.merchantPath, "", "", "", 401)
	f.request(t, "GET", otherPath, f.token, "", "", 403)
	f.request(t, "GET", f.merchantPath, otherToken, "", "", 403)
	_, providerToken, err := persistence.ProvisionProvider(context.Background(), f.admin, []string{"publication:read"}, time.Now().Add(time.Hour))
	must(t, err)
	f.request(t, "GET", f.merchantPath, providerToken, "", "", 401)
	_, err = f.admin.Exec(`UPDATE auth.grants SET scopes=ARRAY['store:write'] WHERE id=$1`, f.principal.ID)
	must(t, err)
	f.request(t, "GET", f.merchantPath, f.token, "", "", 403)
}

func TestManagedStoreDraftSummary(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	path := f.merchantPath + "/" + store.ID.String()
	f.request(t, "PUT", path+"/publication", f.token, "summary-publish-001", `{"expected_generation":0,"revision":1}`, 201)
	var first persistence.StorePage
	must(t, json.Unmarshal(f.request(t, "GET", f.merchantPath, f.token, "", "", 200), &first))
	if len(first.Items) != 1 || first.Items[0].DraftRevision != 1 || first.Items[0].MenuItemCount == nil || *first.Items[0].MenuItemCount != 1 {
		t.Fatalf("first draft summary: %+v", first)
	}
	doc := menu(t)
	items := doc["items"].([]any)
	copyItem := map[string]any{}
	for key, value := range items[0].(map[string]any) {
		copyItem[key] = value
	}
	copyItem["id"] = "second-item"
	doc["items"] = append(items, copyItem)
	f.request(t, "PUT", path+"/offering", f.token, "summary-draft-update-001", offeringBody(t, 1, doc, "ocp.food.menu"), 201)
	var page persistence.StorePage
	must(t, json.Unmarshal(f.request(t, "GET", f.merchantPath, f.token, "", "", 200), &page))
	summary := page.Items[0]
	if summary.DraftRevision != 2 || summary.MenuItemCount == nil || *summary.MenuItemCount != 2 || !summary.UpdatedAt.After(first.Items[0].UpdatedAt) {
		t.Fatalf("summary must describe latest private draft, not older publication: %+v", summary)
	}
	// A summary is store-management metadata, not permission to read the full menu.
	_, err := f.admin.Exec(`UPDATE auth.grants SET scopes=ARRAY['store:read'] WHERE id=$1`, f.principal.ID)
	must(t, err)
	f.request(t, "GET", f.merchantPath, f.token, "", "", 200)
	f.request(t, "GET", path+"/offering", f.token, "", "", 403)
}

// The typed SQL projection must preserve null for non-Food models, rather than
// interpreting them as empty menus. This tests storage mapping, not model support.
func TestManagedStoreNonFoodSummary(t *testing.T) {
	f := setup(t)
	store := preparedStore(t, f)
	_, err := f.admin.Exec(`UPDATE commerce.offering_revisions SET model_id='ocp.shopping.catalog' WHERE merchant_id=$1 AND store_id=$2`, f.principal.MerchantID, store.ID)
	must(t, err)
	page, err := persistence.ListStores(context.Background(), f.app, f.principal.MerchantID, nil, 10, nil)
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].MenuItemCount != nil || page.Items[0].DraftRevision != 1 {
		t.Fatalf("non-Food offering must retain a null menu count: %+v", page)
	}
}

func TestGeneratedStoreListClient(t *testing.T) {
	f := setup(t)
	client, err := httpapi.NewClientWithResponses(f.server.URL)
	must(t, err)
	response, err := client.ListStoresWithResponse(context.Background(), f.principal.MerchantID, nil, func(ctx context.Context, request *http.Request) error {
		request.Header.Set("Authorization", "Bearer "+f.token)
		return nil
	})
	must(t, err)
	if response.StatusCode() != 200 || response.JSON200 == nil || response.JSON200.Items == nil || len(response.JSON200.Items) != 0 {
		t.Fatal("generated client did not decode empty store page")
	}
}
