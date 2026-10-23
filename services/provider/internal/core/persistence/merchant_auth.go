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
	// pgx's array codec is used through its SQL driver.
	var scopesJSON string
	// Keep REST compatible with historical migrations and the original restricted
	// lookup. MCP uses a separate resource filter, never a fallback lookup.
	query := `SELECT id, merchant_id, array_to_json(scopes)::text FROM auth.lookup_grant($1)`
	args := []any{digest[:]}
	if resource == MerchantMCP {
		query = `SELECT id, merchant_id, array_to_json(scopes)::text FROM auth.lookup_resource_grant($1,$2)`
		args = append(args, resource)
	}
	err := db.QueryRowContext(ctx, query, args...).Scan(&p.ID, &p.MerchantID, &scopesJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, fmt.Errorf("credential lookup unavailable: %w", err)
	}
	if err := json.Unmarshal([]byte(scopesJSON), &p.Scopes); err != nil {
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
	if _, err = tx.ExecContext(ctx, `INSERT INTO auth.merchants(id, display_name) VALUES ($1,$2)`, p.MerchantID, name); err != nil {
		return Principal{}, "", err
	}
	encoded, _ := json.Marshal(scopes)
	if _, err = tx.ExecContext(ctx, `INSERT INTO auth.grants(id, merchant_id, token_hash, scopes, expires_at)
  VALUES ($1,$2,$3,ARRAY(SELECT jsonb_array_elements_text($4::jsonb)),$5)`, p.ID, p.MerchantID, digest[:], string(encoded), expiry); err != nil {
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
	query := `INSERT INTO auth.grants(id,merchant_id,token_hash,scopes,expires_at)
	 SELECT $1,id,$3,ARRAY(SELECT jsonb_array_elements_text($4::jsonb)),$5 FROM auth.merchants WHERE id=$2`
	args := []any{p.ID, merchant, digest[:], string(encoded), expiry}
	if resource == MerchantMCP {
		query = `INSERT INTO auth.grants(id,merchant_id,token_hash,scopes,expires_at,resource)
	 SELECT $1,id,$3,ARRAY(SELECT jsonb_array_elements_text($4::jsonb)),$5,$6 FROM auth.merchants WHERE id=$2`
		args = append(args, resource)
	}
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return Principal{}, "", err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Principal{}, "", err
	}
	if count != 1 {
		return Principal{}, "", ErrNotFound
	}
	return p, token, nil
}
