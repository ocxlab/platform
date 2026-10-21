package persistence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Provider grants deliberately have no merchant identity or merchant credential audience.
func AuthenticateProvider(ctx context.Context, db *sql.DB, token string) (Principal, error) {
	if len(token) != 53 || token[:10] != "ocp_admin_" {
		return Principal{}, ErrUnauthenticated
	}
	digest := sha256.Sum256([]byte(token))
	p := Principal{Provider: true}
	var scopes string
	err := db.QueryRowContext(ctx, `SELECT id,array_to_json(scopes)::text FROM auth.lookup_provider_grant($1)`, digest[:]).Scan(&p.ID, &scopes)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	err = json.Unmarshal([]byte(scopes), &p.Scopes)
	return p, err
}

// ProvisionProvider is an explicit administrative/local bootstrap, never an HTTP operation.
func ProvisionProvider(ctx context.Context, db *sql.DB, scopes []string, expiry time.Time) (Principal, string, error) {
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return Principal{}, "", err
	}
	token := "ocp_admin_" + base64.RawURLEncoding.EncodeToString(entropy[:])
	digest := sha256.Sum256([]byte(token))
	p := Principal{ID: uuid.New(), Provider: true, Scopes: scopes}
	encoded, _ := json.Marshal(scopes)
	_, err := db.ExecContext(ctx, `INSERT INTO auth.provider_grants(id,token_hash,scopes,expires_at) VALUES($1,$2,ARRAY(SELECT jsonb_array_elements_text($3::jsonb)),$4)`, p.ID, digest[:], string(encoded), expiry)
	return p, token, err
}
