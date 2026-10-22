package networkcatalog

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/offering/food"
	"ocp.local/platform/internal/persistence"
)

func sample(t *testing.T) (Binding, persistence.Effect) {
	t.Helper()
	b := Binding{ProviderID: "provider-a.lab", MerchantID: uuid.New(), StoreID: uuid.New(), APIBase: "https://provider-a.example/api/v1"}
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b.PublicKey = base64.StdEncoding.EncodeToString(public)
	revision := int64(1)
	payload, err := json.Marshal(persistence.CatalogIntent{Action: "publish", Visible: true, Generation: 1, Revision: &revision, Model: food.Model, DisplayName: "Restaurant", Document: []byte(`{"items":[]}`)})
	if err != nil {
		t.Fatal(err)
	}
	return b, persistence.Effect{ID: uuid.New(), MerchantID: b.MerchantID, StoreID: b.StoreID, SubjectID: b.StoreID, Domain: capability.Food, Kind: "catalog.publish", Payload: payload}
}
func TestProjectionAndIdentity(t *testing.T) {
	b, e := sample(t)
	c, err := Build(b, e)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != CatalogID(b.StoreID, 1) || c.Resources[0].Attributes.OCP.APIBase != b.APIBase {
		t.Fatal("lost discovery routing")
	}
	e.StoreID = uuid.New()
	if _, err = Build(b, e); err == nil {
		t.Fatal("foreign store accepted")
	}
	if b.Validate(false) != nil {
		t.Fatal("valid binding rejected")
	}
	b.APIBase = "http://api:8080/api/v1"
	if b.Validate(false) == nil || b.Validate(true) != nil {
		t.Fatal("HTTP must require local opt-in")
	}
}
func TestPublisherRequiresActualAck(t *testing.T) {
	b, e := sample(t)
	for _, body := range []string{`{}`, `{"status":"COMPLETED","results":[{"status":"REJECTED"}]}`, fmt.Sprintf(`{"status":"COMPLETED","results":[{"catalogId":%q,"status":"ACCEPTED"}]}`, CatalogID(b.StoreID, 1))} {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		b.PublicKey = base64.StdEncoding.EncodeToString(public)
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var value map[string]any
			if json.NewDecoder(r.Body).Decode(&value) != nil {
				t.Error("invalid projection")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		p := Publisher{Endpoint: s.URL, Bindings: map[uuid.UUID]Binding{b.StoreID: b}, PrivateKey: private}
		err = p.Deliver(context.Background(), e)
		s.Close()
		if (err == nil) != strings.Contains(body, `"ACCEPTED"`) {
			t.Fatalf("wrong ACK outcome %s %v", body, err)
		}
	}
}

func TestProjectionSignatureProtectsBindingAndFacts(t *testing.T) {
	b, e := sample(t)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b.PublicKey = base64.StdEncoding.EncodeToString(public)
	c, err := Build(b, e)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := signingBytes(c.Resources[0].Attributes.OCP)
	c.Resources[0].Attributes.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, body))
	if _, err = Verify(c, b); err != nil {
		t.Fatal(err)
	}
	c.Resources[0].Attributes.OCP.Generation++
	if _, err = Verify(c, b); err == nil {
		t.Fatal("changed generation accepted")
	}
	c.Resources[0].Attributes.OCP.Generation--
	c.Resources[0].Attributes.OCP.Document = []byte(`{"items":[{"name":"Forged"}]}`)
	if _, err = Verify(c, b); err == nil {
		t.Fatal("changed offering accepted")
	}
}
