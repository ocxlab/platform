package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/devsetup"
	foodstore "ocp.local/platform/internal/food/persistence"
	"strings"
	"testing"
	"time"
)

func TestDemoSeedAndSelectedMerchantCredentials(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	existing := preparedStore(t, f)
	demo, err := devsetup.EnsureRestaurant(ctx, f.admin)
	must(t, err)
	again, err := devsetup.EnsureRestaurant(ctx, f.admin)
	must(t, err)
	if demo.ID != again.ID || demo.Handle != again.Handle {
		t.Fatal("seed duplicated the shop")
	}
	resolved, err := devsetup.ResolveMerchant(ctx, f.admin, "", demo.Handle)
	must(t, err)
	if resolved != demo.MerchantID {
		t.Fatal("store did not resolve its owner")
	}
	p, token, err := persistence.ProvisionMerchantGrant(ctx, f.admin, resolved, []string{"store:read", "offering:read"}, time.Now().Add(time.Hour))
	must(t, err)
	authenticated, err := persistence.Authenticate(ctx, f.app, token)
	must(t, err)
	if authenticated.MerchantID != demo.MerchantID || p.ID != authenticated.ID {
		t.Fatal("wrong merchant credential")
	}
	path := fmt.Sprintf("/api/v1/merchants/%s/stores/%s", demo.MerchantID, demo.ID)
	f.request(t, "GET", path, token, "", "", 200)
	f.request(t, "GET", path, f.token, "", "", 403)
	f.request(t, "GET", f.merchantPath+"/"+existing.ID.String(), token, "", "", 403)
	public := strings.Replace(path, "/api/v1/", "/api/v1/public/", 1)
	var offering persistence.PublicOffering
	must(t, json.Unmarshal(f.request(t, "GET", public+"/offering", "", "", "", 200), &offering))
	var doc struct {
		Items []any `json:"items"`
	}
	must(t, json.Unmarshal(offering.Document, &doc))
	if len(doc.Items) != 48 {
		t.Fatal("demo menu not published")
	}
	// Required sauce, paid/free extras, quantities, and sizes are server-priced.
	quote := `{"publication_generation":1,"lines":[{"item_id":"doener","variant_id":"large","quantity":2,"options":[{"group_id":"sauce","option_id":"garlic"},{"group_id":"extras","option_id":"cheese"},{"group_id":"extras","option_id":"chilli"}]},{"item_id":"pizza-margherita","variant_id":"large","quantity":1,"options":[{"group_id":"extras","option_id":"cheese"},{"group_id":"extras","option_id":"garlic"}]}]}`
	var priced foodstore.FoodQuote
	must(t, json.Unmarshal(f.request(t, "POST", public+"/food/quotes", "", "", quote, 201), &priced))
	if priced.Total != 3300 {
		t.Fatalf("wrong demo quote: %d", priced.Total)
	}
	f.request(t, "POST", public+"/food/quotes", "", "", `{"publication_generation":1,"lines":[{"item_id":"baklava","variant_id":"regular","quantity":1,"options":[]}]}`, 422)
	f.request(t, "POST", public+"/food/quotes", "", "", `{"publication_generation":1,"lines":[{"item_id":"doener","variant_id":"regular","quantity":1,"options":[]}]}`, 422)
	// Seed again after an edit and withdrawal: never overwrite the walkthrough.
	_, err = f.admin.Exec(`UPDATE commerce.stores SET display_name='Edited demo',published_revision=NULL,public_document=NULL WHERE merchant_id=$1 AND id=$2`, demo.MerchantID, demo.ID)
	must(t, err)
	again, err = devsetup.EnsureRestaurant(ctx, f.admin)
	must(t, err)
	if again.DisplayName != "Edited demo" {
		t.Fatal("seed overwrote edit")
	}
	f.request(t, "GET", public+"/offering", "", "", "", 404)
	if _, err = persistence.GetStore(ctx, f.app, f.principal.MerchantID, existing.ID); err != nil {
		t.Fatal("existing store changed")
	}
	stores, err := devsetup.Stores(ctx, f.admin)
	must(t, err)
	if len(stores) != 2 {
		t.Fatalf("unexpected stores: %d", len(stores))
	}
	if _, err = devsetup.ResolveMerchant(ctx, f.admin, f.principal.MerchantID.String(), demo.Handle); err == nil {
		t.Fatal("ambiguous selector accepted")
	}
	if _, err = devsetup.ResolveMerchant(ctx, f.admin, "", "store-999999"); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatal("unknown store did not fail")
	}
	if _, _, err = persistence.ProvisionMerchantGrant(ctx, f.admin, uuid.New(), nil, time.Now().Add(time.Hour)); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatal("unknown merchant created")
	}
	// Issuing a second grant neither changes identity nor revokes the first one.
	_, _, err = persistence.ProvisionMerchantGrant(ctx, f.admin, demo.MerchantID, []string{"store:read"}, time.Now().Add(time.Hour))
	must(t, err)
	_, err = persistence.Authenticate(ctx, f.app, token)
	must(t, err)
}
