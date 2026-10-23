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
	"ocp.local/platform/internal/core/persistence/sqlgen"
)

// Provider grants deliberately have no merchant identity or merchant credential audience.
func AuthenticateProvider(ctx context.Context, db *sql.DB, token string) (Principal, error) {
	if len(token) != 53 || token[:10] != "ocp_admin_" {
		return Principal{}, ErrUnauthenticated
	}
	digest := sha256.Sum256([]byte(token))
	p := Principal{Provider: true}
	row, err := sqlgen.New(db).LookupProviderGrant(ctx, digest[:])
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	p.ID = row.ID
	err = json.Unmarshal([]byte(row.ScopesJson), &p.Scopes)
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
	err := sqlgen.New(db).InsertProviderGrant(ctx, sqlgen.InsertProviderGrantParams{ID: p.ID, TokenHash: digest[:], ScopesJson: encoded, ExpiresAt: expiry})
	return p, token, err
}
