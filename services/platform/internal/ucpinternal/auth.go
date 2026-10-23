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
	"ocp.local/platform/internal/ucpinternal/persistence/sqlgen"
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
	err := sqlgen.New(admin).InsertGrant(ctx, sqlgen.InsertGrantParams{ID: p.ID, MerchantID: p.MerchantID, StoreID: p.StoreID, CustomerID: p.CustomerID, Domain: string(p.Domain), TokenHash: digest[:], ProfileUri: p.Profile, Profile: sessionkit.JSON(profile), ExpiresAt: expires, Management: management})
	return p, token, err
}
func authenticate(ctx context.Context, db *sql.DB, authorization string) (sessionkit.Principal, Profile, bool, error) {
	var p sessionkit.Principal
	var profile Profile
	token, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok || len(token) < 32 || len(token) > 256 {
		return p, profile, false, sessionkit.ErrNotFound
	}
	digest := sha256.Sum256([]byte(token))
	row, err := sqlgen.New(db).LookupGrant(ctx, digest[:])
	if errors.Is(err, sql.ErrNoRows) {
		err = sessionkit.ErrNotFound
	}
	if err != nil {
		return p, profile, false, err
	}
	p.ID, p.MerchantID, p.StoreID, p.CustomerID = row.ID, row.MerchantID, row.StoreID, row.CustomerID
	p.Domain, p.Profile = capability.Domain(row.Domain), row.ProfileUri
	if p.Profile != ControlledProfileURI || json.Unmarshal(row.Profile, &profile) != nil || !allowedProfile(profile, p.Domain) {
		return p, profile, false, sessionkit.ErrInput
	}
	return p, profile, row.Management, nil
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
