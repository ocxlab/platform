// Package networkcatalog maps owned public offering snapshots to the pinned
// ONIX catalog publisher/crawler seam. It does not implement Beckn checkout.
package networkcatalog

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
)

type Binding struct {
	ProviderID string    `json:"provider_id"`
	MerchantID uuid.UUID `json:"merchant_id"`
	StoreID    uuid.UUID `json:"store_id"`
	APIBase    string    `json:"api_base"`
	PublicKey  string    `json:"public_key"`
}

type Projection struct {
	Binding
	persistence.CatalogIntent
}

type Catalog struct {
	ID         string     `json:"id"`
	Descriptor Descriptor `json:"descriptor"`
	Active     bool       `json:"isActive"`
	Provider   Provider   `json:"provider"`
	Resources  []Resource `json:"resources"`
}
type Descriptor struct {
	Name string `json:"name"`
}
type Provider struct {
	ID         string     `json:"id"`
	Descriptor Descriptor `json:"descriptor"`
}
type Resource struct {
	ID         string     `json:"id"`
	Descriptor Descriptor `json:"descriptor"`
	Attributes struct {
		OCP       Projection `json:"ocp"`
		Signature string     `json:"ocp_signature"`
	} `json:"resourceAttributes"`
}

func CatalogID(store uuid.UUID, generation int64) string {
	return fmt.Sprintf("ocp:%s:%d", store, generation)
}

func (b Binding) Validate(allowLocal bool) error {
	u, err := url.Parse(b.APIBase)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(allowLocal && u.Scheme == "http")) || b.MerchantID == uuid.Nil || b.StoreID == uuid.Nil || b.ProviderID == "" || len(b.ProviderID) > 200 {
		return errors.New("invalid enrolled binding")
	}
	key, err := base64.StdEncoding.DecodeString(b.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("invalid enrolled signing key")
	}
	return nil
}

// Build assigns immutable generation identities. Replaying an old intent cannot
// replace the native artifact for a newer generation. A visibility tombstone is
// distributed like any other snapshot; it contains no unpublished document.
func Build(binding Binding, effect persistence.Effect) (Catalog, error) {
	var intent persistence.CatalogIntent
	if !persistence.SupportedEffect(effect.Domain, effect.Kind) || effect.MerchantID != binding.MerchantID || effect.StoreID != binding.StoreID || effect.SubjectID != binding.StoreID || json.Unmarshal(effect.Payload, &intent) != nil || intent.Generation < 1 {
		return Catalog{}, errors.New("effect does not match enrolled merchant")
	}
	id := CatalogID(binding.StoreID, intent.Generation)
	c := Catalog{ID: id, Descriptor: Descriptor{intent.DisplayName}, Active: true, Provider: Provider{binding.ProviderID, Descriptor{binding.ProviderID}}, Resources: []Resource{{ID: id + "/offering", Descriptor: Descriptor{intent.DisplayName}}}}
	c.Resources[0].Attributes.OCP = Projection{binding, intent}
	return c, nil
}

type Publisher struct {
	Endpoint   string
	Bindings   map[uuid.UUID]Binding
	Client     *http.Client
	PrivateKey ed25519.PrivateKey
}

func (p Publisher) Deliver(ctx context.Context, e persistence.Effect) error {
	binding, ok := p.Bindings[e.StoreID]
	if !ok {
		return errors.New("store has no enrolled network binding")
	}
	catalog, err := Build(binding, e)
	if err != nil {
		return err
	}
	if len(p.PrivateKey) != ed25519.PrivateKeySize || base64.StdEncoding.EncodeToString(p.PrivateKey.Public().(ed25519.PublicKey)) != binding.PublicKey {
		return errors.New("publisher signing key does not match enrollment")
	}
	signed, _ := signingBytes(catalog.Resources[0].Attributes.OCP)
	catalog.Resources[0].Attributes.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(p.PrivateKey, signed))
	value := map[string]any{"context": map[string]string{"action": "catalog/publish"}, "message": map[string]any{"catalogs": []Catalog{catalog}, "publishDirectives": []any{map[string]string{"catalogId": catalog.ID, "catalogType": "REGULAR"}}}}
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("native publisher unavailable")
	}
	defer func() { _ = resp.Body.Close() }()
	var result struct {
		Status  string `json:"status"`
		Results []struct {
			CatalogID string `json:"catalogId"`
			Status    string `json:"status"`
		} `json:"results"`
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(data) > 65536 || resp.StatusCode != 200 || json.Unmarshal(data, &result) != nil || result.Status != "COMPLETED" || len(result.Results) != 1 || result.Results[0].CatalogID != catalog.ID || result.Results[0].Status != "ACCEPTED" {
		return errors.New("native publisher did not acknowledge catalog")
	}
	return nil
}

func signingBytes(p Projection) ([]byte, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	// Canonicalize nested objects too: native storage may reorder JSON keys.
	// This owned integer-amount profile is not a general JSON/JCS standard.
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err = decoder.Decode(&value); err != nil {
		return nil, err
	}
	raw, err = json.Marshal(value)
	return append([]byte("ocp.catalog.projection.v1\n"), raw...), err
}

// Verify does not infer publisher authority from the crawler's transport context.
// The native crawler does not forward original signer identity in that context.
// A separately signed owned projection is checked against explicit enrollment.
func Verify(c Catalog, enrolled Binding) (Projection, error) {
	if len(c.Resources) != 1 {
		return Projection{}, errors.New("one complete offering snapshot required")
	}
	r := c.Resources[0]
	p := r.Attributes.OCP
	if p.Binding != enrolled || c.Provider.ID != enrolled.ProviderID || c.ID != CatalogID(enrolled.StoreID, p.Generation) || r.ID != c.ID+"/offering" || c.Descriptor.Name != p.DisplayName || r.Descriptor.Name != p.DisplayName || !c.Active || p.Generation < 1 {
		return Projection{}, errors.New("catalog does not match enrollment")
	}
	key, err := base64.StdEncoding.DecodeString(enrolled.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return Projection{}, errors.New("invalid enrolled key")
	}
	signature, err := base64.StdEncoding.DecodeString(r.Attributes.Signature)
	if err != nil {
		return Projection{}, errors.New("invalid projection signature")
	}
	body, err := signingBytes(p)
	if err != nil || !ed25519.Verify(key, body, signature) {
		return Projection{}, errors.New("projection signature rejected")
	}
	return p, nil
}
