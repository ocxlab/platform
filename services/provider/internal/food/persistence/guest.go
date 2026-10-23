package persistence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	corestore "ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/database"
	"time"

	"github.com/google/uuid"
)

type GuestSession struct {
	Token     string    `json:"access_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

var ErrGuestCapacity = errors.New("active guest capacity exceeded")

// A guest is an opaque, store-bound checkout grant, not an account. Only its
// digest is retained; the secret never enters quotes, order history or replays.
func CreateGuestSession(ctx context.Context, db *sql.DB, merchant, store uuid.UUID) (GuestSession, error) {
	tx, err := database.TenantTransaction(ctx, db, merchant)
	if err != nil {
		return GuestSession{}, err
	}
	defer tx.Rollback()
	if _, err = corestore.PublicOfferingTx(ctx, tx, merchant, store, true); err != nil {
		return GuestSession{}, err
	}
	// Only abandoned grants are disposable. Retain order actors and replay
	// references; neither expired terms nor expired credentials erase a purchase.
	if _, err = tx.ExecContext(ctx, `DELETE FROM commerce.food_quotes WHERE merchant_id=$1 AND store_id=$2 AND id IN
 (SELECT id FROM commerce.food_quotes WHERE merchant_id=$1 AND store_id=$2 AND expires_at<=clock_timestamp() ORDER BY expires_at LIMIT 1000)`, merchant, store); err != nil {
		return GuestSession{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM commerce.guest_grants WHERE merchant_id=$1 AND id IN
 (SELECT g.id FROM commerce.guest_grants g WHERE g.merchant_id=$1 AND g.store_id=$2
 AND (g.expires_at<=clock_timestamp() OR g.revoked)
 AND NOT EXISTS(SELECT FROM commerce.food_quotes q WHERE q.merchant_id=g.merchant_id AND q.guest_id=g.id)
 AND NOT EXISTS(SELECT FROM commerce.food_orders o WHERE o.merchant_id=g.merchant_id AND o.guest_id=g.id)
 AND NOT EXISTS(SELECT FROM commerce.guest_request_replays r WHERE r.merchant_id=g.merchant_id AND r.grant_id=g.id)
 ORDER BY g.expires_at LIMIT 1000)`, merchant, store); err != nil {
		return GuestSession{}, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM commerce.guest_grants WHERE merchant_id=$1 AND store_id=$2 AND expires_at>clock_timestamp() AND NOT revoked`, merchant, store).Scan(&count); err != nil {
		return GuestSession{}, err
	}
	if count >= 1000 {
		return GuestSession{}, ErrGuestCapacity
	}
	var entropy [32]byte
	if _, err = rand.Read(entropy[:]); err != nil {
		return GuestSession{}, err
	}
	result := GuestSession{Token: "ocg_" + base64.RawURLEncoding.EncodeToString(entropy[:])}
	digest := sha256.Sum256([]byte(result.Token))
	err = tx.QueryRowContext(ctx, `INSERT INTO commerce.guest_grants(merchant_id,store_id,id,token_hash,expires_at) VALUES($1,$2,$3,$4,clock_timestamp()+interval '7 days') RETURNING expires_at`, merchant, store, uuid.New(), digest[:]).Scan(&result.ExpiresAt)
	if err != nil {
		return GuestSession{}, err
	}
	return result, tx.Commit()
}
