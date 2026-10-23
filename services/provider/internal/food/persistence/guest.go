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
	"ocp.local/platform/internal/food/persistence/sqlgen"
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
	queries := sqlgen.New(tx)
	if err = queries.PruneExpiredFoodQuotes(ctx, sqlgen.PruneExpiredFoodQuotesParams{MerchantID: merchant, StoreID: store}); err != nil {
		return GuestSession{}, err
	}
	if err = queries.PruneAbandonedGuestGrants(ctx, sqlgen.PruneAbandonedGuestGrantsParams{MerchantID: merchant, StoreID: store}); err != nil {
		return GuestSession{}, err
	}
	count, err := queries.CountActiveGuestGrants(ctx, sqlgen.CountActiveGuestGrantsParams{MerchantID: merchant, StoreID: store})
	if err != nil {
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
	result.ExpiresAt, err = queries.InsertGuestGrant(ctx, sqlgen.InsertGuestGrantParams{MerchantID: merchant, StoreID: store, ID: uuid.New(), TokenHash: digest[:]})
	if err != nil {
		return GuestSession{}, err
	}
	return result, tx.Commit()
}
