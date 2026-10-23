package persistence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence/sqlgen"
	"time"
)

type Principal struct {
	ID         uuid.UUID
	MerchantID uuid.UUID
	Scopes     []string
	Provider   bool
	Guest      bool
	StoreID    uuid.UUID
}

func (p Principal) Allows(scope string) bool {
	for _, available := range p.Scopes {
		if available == scope {
			return true
		}
	}
	return false
}

// AuthorizesMerchant is shared by REST and agent application operations. Provider
// administration and guest checkout have separate policies and cannot use it.
func (p Principal) AuthorizesMerchant(merchant uuid.UUID, scopes ...string) bool {
	if p.Provider || p.Guest || p.ID == uuid.Nil || merchant == uuid.Nil || p.MerchantID != merchant {
		return false
	}
	for _, scope := range scopes {
		if !p.Allows(scope) {
			return false
		}
	}
	return true
}

const MerchantREST = "merchant-rest"
const MerchantMCP = "merchant-mcp"

var ErrUnauthenticated = errors.New("invalid credential")

func Authenticate(ctx context.Context, db *sql.DB, token string) (Principal, error) {
	return AuthenticateResource(ctx, db, token, MerchantREST)
}

func AuthenticateResource(ctx context.Context, db *sql.DB, token, resource string) (Principal, error) {
	if resource != MerchantREST && resource != MerchantMCP {
		return Principal{}, ErrUnauthenticated
	}
	if len(token) != 47 || token[:4] != "ocp_" {
		return Principal{}, ErrUnauthenticated
	}
	digest := sha256.Sum256([]byte(token))
	var p Principal
	// Keep REST compatible with historical migrations and the original restricted
	// lookup. MCP uses a separate resource filter, never a fallback lookup.
	queries := sqlgen.New(db)
	var row sqlgen.LookupMerchantGrantRow
	var err error
	if resource == MerchantMCP {
		var bound sqlgen.LookupMerchantResourceGrantRow
		bound, err = queries.LookupMerchantResourceGrant(ctx, sqlgen.LookupMerchantResourceGrantParams{Candidate: digest[:], RequestedResource: resource})
		row = sqlgen.LookupMerchantGrantRow(bound)
	} else {
		row, err = queries.LookupMerchantGrant(ctx, digest[:])
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, fmt.Errorf("credential lookup unavailable: %w", err)
	}
	p.ID, p.MerchantID = row.ID, row.MerchantID
	if err := json.Unmarshal([]byte(row.ScopesJson), &p.Scopes); err != nil {
		return Principal{}, err
	}
	return p, nil
}

func Provision(ctx context.Context, db *sql.DB, name string, scopes []string, expiry time.Time) (Principal, string, error) {
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return Principal{}, "", err
	}
	token := "ocp_" + base64.RawURLEncoding.EncodeToString(entropy[:])
	digest := sha256.Sum256([]byte(token))
	p := Principal{ID: uuid.New(), MerchantID: uuid.New(), Scopes: scopes}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Principal{}, "", err
	}
	defer tx.Rollback()
	queries := sqlgen.New(tx)
	if err = queries.InsertMerchant(ctx, sqlgen.InsertMerchantParams{ID: p.MerchantID, DisplayName: name}); err != nil {
		return Principal{}, "", err
	}
	encoded, _ := json.Marshal(scopes)
	if err = queries.InsertMerchantGrant(ctx, sqlgen.InsertMerchantGrantParams{ID: p.ID, MerchantID: p.MerchantID, TokenHash: digest[:], ScopesJson: encoded, ExpiresAt: expiry}); err != nil {
		return Principal{}, "", err
	}
	if err = tx.Commit(); err != nil {
		return Principal{}, "", err
	}
	return p, token, nil
}

// ProvisionMerchantGrant is an explicit administrative bootstrap, not public
// account recovery. It creates a new credential without altering existing grants.
func ProvisionMerchantGrant(ctx context.Context, db *sql.DB, merchant uuid.UUID, scopes []string, expiry time.Time) (Principal, string, error) {
	return provisionMerchantGrant(ctx, db, merchant, scopes, expiry, MerchantREST)
}

// ProvisionMerchantMCPGrant is a localhost bootstrap, not an OAuth token endpoint.
func ProvisionMerchantMCPGrant(ctx context.Context, db *sql.DB, merchant uuid.UUID, scopes []string, expiry time.Time) (Principal, string, error) {
	return provisionMerchantGrant(ctx, db, merchant, scopes, expiry, MerchantMCP)
}

func provisionMerchantGrant(ctx context.Context, db *sql.DB, merchant uuid.UUID, scopes []string, expiry time.Time, resource string) (Principal, string, error) {
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return Principal{}, "", err
	}
	token := "ocp_" + base64.RawURLEncoding.EncodeToString(entropy[:])
	digest := sha256.Sum256([]byte(token))
	p := Principal{ID: uuid.New(), MerchantID: merchant, Scopes: scopes}
	encoded, err := json.Marshal(scopes)
	if err != nil {
		return Principal{}, "", err
	}
	// Preserve the REST bootstrap for historical migration integration tests.
	queries := sqlgen.New(db)
	var count int64
	if resource == MerchantMCP {
		count, err = queries.ProvisionMerchantResourceGrant(ctx, sqlgen.ProvisionMerchantResourceGrantParams{ID: p.ID, MerchantID: merchant, TokenHash: digest[:], ScopesJson: encoded, ExpiresAt: expiry, Resource: resource})
	} else {
		count, err = queries.ProvisionMerchantGrant(ctx, sqlgen.ProvisionMerchantGrantParams{ID: p.ID, MerchantID: merchant, TokenHash: digest[:], ScopesJson: encoded, ExpiresAt: expiry})
	}
	if err != nil {
		return Principal{}, "", err
	}
	if count != 1 {
		return Principal{}, "", ErrNotFound
	}
	return p, token, nil
}
