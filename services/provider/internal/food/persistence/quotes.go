package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	corestore "ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/database"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/offering"
)

var ErrQuoteCapacity = errors.New("active quote capacity exceeded")

type QuoteCalculation struct {
	ExpiresAt time.Time
	Note      string
	Lines     json.RawMessage
	Total     int64
}
type FoodQuote struct {
	Note       string          `json:"note,omitempty"`
	ID         uuid.UUID       `json:"id"`
	MerchantID uuid.UUID       `json:"merchant_id"`
	StoreID    uuid.UUID       `json:"store_id"`
	Generation int64           `json:"publication_generation"`
	Revision   int64           `json:"revision"`
	Model      offering.Model  `json:"model"`
	Currency   string          `json:"currency"`
	Lines      json.RawMessage `json:"lines"`
	Total      int64           `json:"total_minor"`
	CreatedAt  time.Time       `json:"created_at"`
	ExpiresAt  time.Time       `json:"expires_at"`
}

func CreateFoodQuote(ctx context.Context, db *sql.DB, merchant, store uuid.UUID, expected int64, calculate func(corestore.PublicOffering) (QuoteCalculation, error)) (FoodQuote, error) {
	return createFoodQuote(ctx, db, merchant, store, expected, nil, calculate)
}

func CreateGuestFoodQuote(ctx context.Context, db *sql.DB, guest corestore.Principal, expected int64, calculate func(corestore.PublicOffering) (QuoteCalculation, error)) (FoodQuote, error) {
	return createFoodQuote(ctx, db, guest.MerchantID, guest.StoreID, expected, &guest, calculate)
}

func createFoodQuote(ctx context.Context, db *sql.DB, merchant, store uuid.UUID, expected int64, guest *corestore.Principal, calculate func(corestore.PublicOffering) (QuoteCalculation, error)) (FoodQuote, error) {
	q := FoodQuote{}
	tx, err := database.TenantTransaction(ctx, db, merchant)
	if err != nil {
		return q, err
	}
	defer tx.Rollback()
	var guestID any
	if guest != nil {
		if err := corestore.CheckGuest(ctx, tx, *guest); err != nil {
			return q, err
		}
		guestID = guest.ID
	}
	p, err := corestore.PublicOfferingTx(ctx, tx, merchant, store, true)
	if err != nil {
		return q, err
	}
	ordering, err := foodOrdering(ctx, tx, merchant, store, false)
	if err != nil {
		return q, err
	}
	if !ordering.AcceptingOrders {
		return q, ErrOrderingPaused
	}
	if p.Generation != expected {
		return q, &corestore.PublicationConflict{Current: p.Generation}
	}
	var evaluatedAt time.Time
	if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&evaluatedAt); err != nil {
		return q, err
	}
	p.EvaluatedAt = evaluatedAt
	calculation, err := calculate(p)
	if err != nil {
		return q, err
	}
	// Bound ephemeral anonymous writes per store. Expired snapshots are not purchase
	// history. A later placement transaction must reject expired terms regardless.
	if _, err = tx.ExecContext(ctx, `DELETE FROM commerce.food_quotes WHERE merchant_id=$1 AND store_id=$2 AND id IN
 (SELECT id FROM commerce.food_quotes WHERE merchant_id=$1 AND store_id=$2 AND expires_at<=clock_timestamp() ORDER BY expires_at LIMIT 1000)`, merchant, store); err != nil {
		return q, err
	}
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM commerce.food_quotes WHERE merchant_id=$1 AND store_id=$2 AND expires_at>clock_timestamp()`, merchant, store).Scan(&active); err != nil {
		return q, err
	}
	if active >= 1000 {
		return q, ErrQuoteCapacity
	}
	q = FoodQuote{ID: uuid.New(), MerchantID: merchant, StoreID: store, Generation: p.Generation, Revision: p.Revision, Model: p.Model, Currency: "EUR", Lines: calculation.Lines, Total: calculation.Total, Note: calculation.Note}
	q.CreatedAt = evaluatedAt
	q.ExpiresAt = q.CreatedAt.Add(10 * time.Minute)
	if !calculation.ExpiresAt.IsZero() && calculation.ExpiresAt.Before(q.ExpiresAt) {
		q.ExpiresAt = calculation.ExpiresAt
	}
	if !q.ExpiresAt.After(q.CreatedAt) {
		return q, ErrQuoteExpired
	}
	body, err := json.Marshal(q)
	if err != nil {
		return q, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO commerce.food_quotes(merchant_id,store_id,id,body,expires_at,guest_id) VALUES($1,$2,$3,$4,$5,$6)`, merchant, store, q.ID, string(body), q.ExpiresAt, guestID)
	if err != nil {
		return q, err
	}
	return q, tx.Commit()
}
