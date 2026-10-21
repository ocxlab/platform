package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/persistence"
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
	seen := map[uuid.UUID]bool{}
	path := f.merchantPath + "?limit=1"
	var previous string
	for i := 0; i < 2; i++ {
		page = readPage(path, f.token)
		if len(page.Items) != 1 {
			t.Fatalf("page %d: %+v", i, page)
		}
		store := page.Items[0]
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
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=wrong", "?after=not-a-uuid"} {
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
