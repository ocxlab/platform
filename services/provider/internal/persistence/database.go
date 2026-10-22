// Package persistence owns foundation SQL access and explicit migrations.
package persistence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, errors.New("database URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, errors.New("invalid database connection configuration")
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.New("database connection unavailable")
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sub)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

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

func TenantTransaction(ctx context.Context, db *sql.DB, merchant uuid.UUID) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT set_config('ocp.merchant_id',$1,true)`, merchant.String()); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

// CheckRuntimeRole rejects migration/administrative connections in the API.
func CheckRuntimeRole(ctx context.Context, db *sql.DB) error {
	var safe bool
	err := db.QueryRowContext(ctx, `SELECT NOT rolsuper AND NOT rolbypassrls
 AND NOT rolcreaterole AND NOT rolcreatedb AND rolname = 'ocp_app'
 AND NOT EXISTS (SELECT FROM pg_auth_members WHERE member=current_user::regrole)
 AND NOT EXISTS (SELECT FROM pg_roles elevated WHERE
   (elevated.rolsuper OR elevated.rolbypassrls OR elevated.rolcreaterole OR elevated.rolcreatedb)
   AND pg_has_role(current_user::regrole, elevated.oid, 'MEMBER'))
 AND NOT EXISTS (SELECT FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE n.nspname IN ('auth','commerce','payment') AND pg_has_role(current_user::regrole,c.relowner,'MEMBER'))
 AND NOT EXISTS (SELECT FROM pg_namespace n WHERE n.nspname IN ('auth','commerce','payment')
   AND pg_has_role(current_user::regrole,n.nspowner,'MEMBER'))
 AND NOT EXISTS (SELECT FROM pg_database d WHERE d.datname=current_database()
   AND pg_has_role(current_user::regrole,d.datdba,'MEMBER'))
 FROM pg_roles WHERE rolname = current_user`).Scan(&safe)
	if err != nil {
		return err
	}
	if !safe {
		return errors.New("API requires the restricted ocp_app role")
	}
	return nil
}
