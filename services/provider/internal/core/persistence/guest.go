package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

func AuthenticateGuest(ctx context.Context, db *sql.DB, token string) (Principal, error) {
	if len(token) != 47 || token[:4] != "ocg_" {
		return Principal{}, ErrUnauthenticated
	}
	digest := sha256.Sum256([]byte(token))
	p := Principal{Guest: true, Scopes: []string{"quote:create", "order:create", "order:read", "order:cancel"}}
	err := db.QueryRowContext(ctx, `SELECT id,merchant_id,store_id FROM auth.lookup_guest_grant($1)`, digest[:]).Scan(&p.ID, &p.MerchantID, &p.StoreID)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	return p, err
}

// Recheck at the application transaction boundary, including before replay.
func CheckGuest(ctx context.Context, tx *sql.Tx, p Principal) error {
	if !p.Guest {
		return ErrUnauthenticated
	}
	var id uuid.UUID
	err := tx.QueryRowContext(ctx, `SELECT id FROM commerce.guest_grants WHERE merchant_id=$1 AND store_id=$2 AND id=$3 AND NOT revoked AND expires_at>clock_timestamp() FOR SHARE`, p.MerchantID, p.StoreID, p.ID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnauthenticated
	}
	return err
}
