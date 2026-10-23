package integration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"ocp.local/platform/cmd/api/transport/httpapi"
	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/offering"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/discovery"
	"ocp.local/platform/internal/food/offering"
)

func networkModels(t *testing.T) *offering.Registry {
	t.Helper()
	spec, err := httpapi.GetSwagger()
	must(t, err)
	return offering.NewRegistry(map[offering.Model]offering.Validator{food.Model: food.Validator(spec.Components.Schemas["FoodMenu"].Value)}, map[offering.Model]offering.Validator{food.Model: food.Project})
}

func TestVerifiedDiscoveryGenerationAndBinding(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	must(t, err)
	b := networkcatalog.Binding{ProviderID: "provider-a.lab", MerchantID: uuid.New(), StoreID: uuid.New(), APIBase: "https://provider-a.example/api/v1", PublicKey: base64.StdEncoding.EncodeToString(pub)}
	_, err = f.admin.Exec(`INSERT INTO discovery.bindings(store_id,merchant_id,provider_id,api_base,public_key) VALUES($1,$2,$3,$4,$5)`, b.StoreID, b.MerchantID, b.ProviderID, b.APIBase, b.PublicKey)
	must(t, err)
	index := networkcatalog.Index{DB: f.app, Bindings: map[uuid.UUID]networkcatalog.Binding{b.StoreID: b}, Models: networkModels(t)}
	var captured networkcatalog.Catalog
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Message struct {
				Catalogs []networkcatalog.Catalog `json:"catalogs"`
			} `json:"message"`
		}
		if json.NewDecoder(r.Body).Decode(&v) != nil || len(v.Message.Catalogs) != 1 {
			w.WriteHeader(400)
			return
		}
		captured = v.Message.Catalogs[0]
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "COMPLETED", "results": []any{map[string]string{"catalogId": captured.ID, "status": "ACCEPTED"}}})
	}))
	defer s.Close()
	publisher := networkcatalog.Publisher{Endpoint: s.URL, Bindings: index.Bindings, PrivateKey: priv}
	revision := int64(1)
	raw, err := json.Marshal(menu(t))
	must(t, err)
	intent := persistence.CatalogIntent{Action: "publish", Visible: true, Generation: 1, Revision: &revision, Model: food.Model, DisplayName: "Network restaurant", Document: raw}
	publish := func() networkcatalog.Catalog {
		t.Helper()
		payload, err := json.Marshal(intent)
		must(t, err)
		must(t, publisher.Deliver(ctx, persistence.Effect{MerchantID: b.MerchantID, StoreID: b.StoreID, SubjectID: b.StoreID, Domain: capability.Food, Kind: "catalog.publish", Payload: payload}))
		return captured
	}
	first := publish()
	must(t, index.Accept(ctx, first))
	must(t, index.Accept(ctx, first))
	rows, err := index.Search(ctx, "restaurant")
	must(t, err)
	if len(rows) != 1 || rows[0].Generation != 1 {
		t.Fatal("missing discovered offering")
	}
	// A changed payload under the same generation is not an idempotent replay,
	// even when its owner correctly signs it.
	intent.DisplayName = "Changed same generation"
	conflict := publish()
	if index.Accept(ctx, conflict) == nil {
		t.Fatal("conflicting generation accepted")
	}
	intent.Generation = 2
	intent.Action = "withdraw"
	intent.Visible = false
	intent.Document = []byte(`null`)
	withdrawn := publish()
	must(t, index.Accept(ctx, withdrawn))
	must(t, index.Accept(ctx, first))
	rows, err = index.Search(ctx, "")
	must(t, err)
	if len(rows) != 0 {
		t.Fatal("late old snapshot resurrected withdrawn store")
	}
	intent.Generation = 3
	intent.Action = "publish"
	intent.Visible = true
	intent.Document = raw
	current := publish()
	must(t, index.Accept(ctx, current))
	rows, err = index.Search(ctx, "")
	must(t, err)
	if len(rows) != 1 || rows[0].Generation != 3 {
		t.Fatal("republish not discovered")
	}
	for _, mutate := range []func(*networkcatalog.Catalog){
		func(c *networkcatalog.Catalog) {
			c.Resources[0].Attributes.OCP.APIBase = "https://wrong.example/api/v1"
		},
		func(c *networkcatalog.Catalog) { c.Resources[0].Attributes.OCP.StoreID = uuid.New() },
		func(c *networkcatalog.Catalog) {
			c.Resources[0].Attributes.Signature = base64.StdEncoding.EncodeToString(make([]byte, 64))
		},
	} {
		copyRaw, _ := json.Marshal(current)
		var changed networkcatalog.Catalog
		must(t, json.Unmarshal(copyRaw, &changed))
		mutate(&changed)
		if index.Accept(ctx, changed) == nil {
			t.Fatal("forged projection accepted")
		}
	}
	// Freshness bounds are about discovery, not permission to transact.
	_, err = f.admin.Exec(`UPDATE discovery.catalogs SET observed_at=clock_timestamp()-interval '25 hours'`)
	must(t, err)
	rows, err = index.Search(ctx, "")
	must(t, err)
	if len(rows) != 0 {
		t.Fatal("stale discovery remains visible")
	}
	must(t, index.Accept(ctx, current))
	rows, err = index.Search(ctx, "")
	must(t, err)
	if len(rows) != 1 {
		t.Fatal("authenticated duplicate did not renew discovery freshness")
	}
}
