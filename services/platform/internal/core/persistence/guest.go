package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"

	"ocp.local/platform/internal/core/persistence/sqlgen"
)

func AuthenticateGuest(ctx context.Context, db *sql.DB, token string) (Principal, error) {
	if len(token) != 47 || token[:4] != "ocg_" {
		return Principal{}, ErrUnauthenticated
	}
	digest := sha256.Sum256([]byte(token))
	p := Principal{Guest: true, Scopes: []string{"quote:create", "order:create", "order:read", "order:cancel"}}
	row, err := sqlgen.New(db).LookupGuestGrant(ctx, digest[:])
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	p.ID, p.MerchantID, p.StoreID = row.ID, row.MerchantID, row.StoreID
	return p, err
}

// Recheck at the application transaction boundary, including before replay.
func CheckGuest(ctx context.Context, tx *sql.Tx, p Principal) error {
	if !p.Guest {
		return ErrUnauthenticated
	}
	_, err := sqlgen.New(tx).LockActiveGuestGrant(ctx, sqlgen.LockActiveGuestGrantParams{MerchantID: p.MerchantID, StoreID: p.StoreID, ID: p.ID})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnauthenticated
	}
	return err
}
