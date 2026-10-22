// Package ucpinternal implements an isolated, controlled internal UCP cohort.
// It is never mounted on the public Food listener.
package ucpinternal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/sessionkit"
)

type Profile struct {
	UCP struct {
		Version         string                         `json:"version"`
		Services        map[string][]sessionkit.Entity `json:"services"`
		Capabilities    map[string][]sessionkit.Entity `json:"capabilities"`
		PaymentHandlers map[string][]sessionkit.Entity `json:"payment_handlers"`
	} `json:"ucp"`
}

var agentHeader = regexp.MustCompile(`^profile="(https://[^"\\]+/\.well-known/ucp)"$`)

const ControlledProfileURI = "https://agent.example/.well-known/ucp"

func validProfileURI(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == "/.well-known/ucp" && u.RawPath == "" && u.Port() == ""
}
func validProfile(p Profile) bool {
	if p.UCP.Version != sessionkit.Version || p.UCP.Services == nil || p.UCP.Capabilities == nil || p.UCP.PaymentHandlers == nil {
		return false
	}
	for name, entities := range p.UCP.Capabilities {
		if name != "dev.ucp.shopping.checkout" && name != "dev.ucp.lodging.booking" {
			return false
		}
		for _, e := range entities {
			u, err := url.Parse(e.Schema)
			if err != nil || u.Scheme != "https" || u.Host != "ucp.dev" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || e.Version != sessionkit.Version {
				return false
			}
			expected := "/schemas/shopping/checkout.json"
			if name == "dev.ucp.lodging.booking" {
				expected = "/schemas/lodging/booking.json"
			}
			if u.Path != expected {
				return false
			}
		}
	}
	return true
}

// The controlled cohort admits only server-defined capability/handler sets.
// Provisioning a URL does not authorize arbitrary services or payment handlers.
func allowedProfile(p Profile, domain capability.Domain) bool {
	if !validProfile(p) || len(p.UCP.Capabilities[sessionkit.Capability(domain)]) != 1 {
		return false
	}
	return reflect.DeepEqual(p, ControlledProfile(domain)) || reflect.DeepEqual(p, ControlledProfile(capability.Shopping, capability.Lodging))
}

// Provision is admin-only bootstrap. Only a digest is retained; the raw token
// is returned once to its explicit local caller, never logged by the server.
func Provision(ctx context.Context, admin *sql.DB, p sessionkit.Principal, profile Profile, management bool, expires time.Time) (sessionkit.Principal, string, error) {
	if (p.Domain != capability.Shopping && p.Domain != capability.Lodging) || p.MerchantID == uuid.Nil || p.StoreID == uuid.Nil || p.CustomerID == uuid.Nil || p.Profile != ControlledProfileURI || !allowedProfile(profile, p.Domain) || !expires.After(time.Now()) || expires.After(time.Now().Add(24*time.Hour)) {
		return p, "", sessionkit.ErrInput
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return p, "", err
	}
	token := "ocp_internal_" + base64.RawURLEncoding.EncodeToString(secret)
	digest := sha256.Sum256([]byte(token))
	p.ID = uuid.New()
	_, err := admin.ExecContext(ctx, `INSERT INTO commerce_internal.grants(id,merchant_id,store_id,customer_id,domain,audience,token_hash,profile_uri,profile,expires_at,management) VALUES($1,$2,$3,$4,$5,'ucp-internal',$6,$7,$8,$9,$10)`, p.ID, p.MerchantID, p.StoreID, p.CustomerID, p.Domain, digest[:], p.Profile, string(sessionkit.JSON(profile)), expires, management)
	return p, token, err
}
func authenticate(ctx context.Context, db *sql.DB, authorization string) (sessionkit.Principal, Profile, bool, error) {
	var p sessionkit.Principal
	var profile Profile
	var raw []byte
	var management bool
	token, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok || len(token) < 32 || len(token) > 256 {
		return p, profile, false, sessionkit.ErrNotFound
	}
	digest := sha256.Sum256([]byte(token))
	err := db.QueryRowContext(ctx, `SELECT id,merchant_id,store_id,customer_id,domain,profile_uri,profile,management FROM commerce_internal.lookup_grant($1)`, digest[:]).Scan(&p.ID, &p.MerchantID, &p.StoreID, &p.CustomerID, &p.Domain, &p.Profile, &raw, &management)
	if errors.Is(err, sql.ErrNoRows) {
		err = sessionkit.ErrNotFound
	}
	if err != nil {
		return p, profile, false, err
	}
	if p.Profile != ControlledProfileURI || json.Unmarshal(raw, &profile) != nil || !allowedProfile(profile, p.Domain) {
		return p, profile, false, sessionkit.ErrInput
	}
	return p, profile, management, nil
}

// ControlledProfile is a static, allowlisted fixture. Requests never trigger a
// server-side fetch of caller-supplied URLs, schemas, or redirects.
func ControlledProfile(domains ...capability.Domain) Profile {
	var p Profile
	p.UCP.Version = sessionkit.Version
	p.UCP.Services = map[string][]sessionkit.Entity{}
	p.UCP.Capabilities = map[string][]sessionkit.Entity{}
	p.UCP.PaymentHandlers = map[string][]sessionkit.Entity{}
	for _, d := range domains {
		leaf := "checkout"
		if d == capability.Lodging {
			leaf = "booking"
		}
		p.UCP.Capabilities[sessionkit.Capability(d)] = []sessionkit.Entity{{Version: sessionkit.Version, Schema: "https://ucp.dev/schemas/" + string(d) + "/" + leaf + ".json", Spec: "https://ucp.dev/specification/" + string(d)}}
	}
	p.UCP.PaymentHandlers[sessionkit.PayLaterHandler] = []sessionkit.Entity{{ID: sessionkit.PayLaterID, Version: sessionkit.Version, Schema: "https://internal.ocplab.org/schemas/internal/pay-later.json", Spec: "https://internal.ocplab.org/schemas/internal/pay-later.json"}}
	return p
}
