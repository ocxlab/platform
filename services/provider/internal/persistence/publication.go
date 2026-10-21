package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/offering"
)

type Publication struct {
	MerchantID uuid.UUID `json:"merchant_id"`
	StoreID    uuid.UUID `json:"store_id"`
	Generation int64     `json:"generation"`
	Revision   *int64    `json:"revision"`
	Suspended  bool      `json:"suspended"`
}
type PublicationConflict struct{ Current int64 }

var ErrQuoteCapacity = errors.New("active quote capacity exceeded")

func (e *PublicationConflict) Error() string { return "publication generation conflict" }

func publication(ctx context.Context, tx *sql.Tx, merchant, store uuid.UUID, lock bool) (Publication, error) {
	p := Publication{MerchantID: merchant, StoreID: store}
	query := `SELECT publication_generation,published_revision,publication_suspended FROM commerce.stores WHERE merchant_id=$1 AND id=$2`
	if lock {
		query += ` FOR UPDATE`
	}
	err := tx.QueryRowContext(ctx, query, merchant, store).Scan(&p.Generation, &p.Revision, &p.Suspended)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}
func GetPublication(ctx context.Context, db *sql.DB, merchant, store uuid.UUID) (Publication, error) {
	tx, err := TenantTransaction(ctx, db, merchant)
	if err != nil {
		return Publication{}, err
	}
	defer tx.Rollback()
	p, err := publication(ctx, tx, merchant, store, false)
	if err != nil {
		return p, err
	}
	return p, tx.Commit()
}

// ChangePublication serializes all visibility changes on the store's existing lock.
// Projection is a model-owned conversion, never an unfiltered draft copy.
func ChangePublication(ctx context.Context, tx *sql.Tx, actor Principal, store uuid.UUID, expected int64, revision *int64, suspension *bool, reason string, requestID uuid.UUID, project func(offering.Model, json.RawMessage) (json.RawMessage, error)) (Publication, error) {
	p, err := publication(ctx, tx, actor.MerchantID, store, true)
	if err != nil {
		return p, err
	}
	if p.Generation != expected {
		return p, &PublicationConflict{p.Generation}
	}
	action, kind := "withdraw", "merchant"
	if suspension != nil {
		if !actor.Provider {
			return p, errors.New("provider principal required")
		}
		kind = "provider"
		action = "restore"
		if *suspension {
			action = "suspend"
		}
		p.Suspended = *suspension
		_, err = tx.ExecContext(ctx, `UPDATE commerce.stores SET publication_suspended=$3,publication_generation=publication_generation+1 WHERE merchant_id=$1 AND id=$2`, actor.MerchantID, store, p.Suspended)
	} else {
		if actor.Provider {
			return p, errors.New("merchant principal required")
		}
		var projected any
		if revision != nil {
			var model offering.Model
			var doc json.RawMessage
			err = tx.QueryRowContext(ctx, `SELECT model_id,model_version,document FROM commerce.offering_revisions WHERE merchant_id=$1 AND store_id=$2 AND revision=$3`, actor.MerchantID, store, *revision).Scan(&model.ID, &model.Version, &doc)
			if errors.Is(err, sql.ErrNoRows) {
				return p, ErrNotFound
			}
			if err != nil {
				return p, err
			}
			public, err := project(model, doc)
			if err != nil {
				return p, err
			}
			projected = string(public)
			action = "publish"
		}
		p.Revision = revision
		_, err = tx.ExecContext(ctx, `UPDATE commerce.stores SET published_revision=$3,public_document=$4,publication_generation=publication_generation+1 WHERE merchant_id=$1 AND id=$2`, actor.MerchantID, store, revision, projected)
	}
	if err != nil {
		return p, err
	}
	p.Generation++
	_, err = tx.ExecContext(ctx, `INSERT INTO commerce.publication_audit(merchant_id,store_id,generation,actor_id,actor_kind,action,reason,revision,suspended,request_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, actor.MerchantID, store, p.Generation, actor.ID, kind, action, reason, p.Revision, p.Suspended, requestID)
	return p, err
}

type PublicOffering struct {
	MerchantID  uuid.UUID       `json:"merchant_id"`
	StoreID     uuid.UUID       `json:"store_id"`
	DisplayName string          `json:"display_name"`
	Generation  int64           `json:"publication_generation"`
	Revision    int64           `json:"revision"`
	Model       offering.Model  `json:"model"`
	Document    json.RawMessage `json:"document"`
}

func publicOffering(ctx context.Context, tx *sql.Tx, merchant, store uuid.UUID, lock bool) (PublicOffering, error) {
	p := PublicOffering{MerchantID: merchant, StoreID: store}
	query := `SELECT s.display_name,s.publication_generation,s.published_revision,r.model_id,r.model_version,s.public_document
 FROM commerce.stores s JOIN commerce.offering_revisions r ON (s.merchant_id,s.id,s.published_revision)=(r.merchant_id,r.store_id,r.revision)
 WHERE s.merchant_id=$1 AND s.id=$2 AND NOT s.publication_suspended`
	if lock {
		query += ` FOR UPDATE OF s`
	}
	err := tx.QueryRowContext(ctx, query, merchant, store).Scan(&p.DisplayName, &p.Generation, &p.Revision, &p.Model.ID, &p.Model.Version, &p.Document)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}
func GetPublicOffering(ctx context.Context, db *sql.DB, merchant, store uuid.UUID) (PublicOffering, error) {
	tx, err := TenantTransaction(ctx, db, merchant)
	if err != nil {
		return PublicOffering{}, err
	}
	defer tx.Rollback()
	p, err := publicOffering(ctx, tx, merchant, store, false)
	if err != nil {
		return p, err
	}
	return p, tx.Commit()
}

type QuoteCalculation struct {
	Lines json.RawMessage
	Total int64
}
type FoodQuote struct {
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

func CreateFoodQuote(ctx context.Context, db *sql.DB, merchant, store uuid.UUID, expected int64, calculate func(PublicOffering) (QuoteCalculation, error)) (FoodQuote, error) {
	return createFoodQuote(ctx, db, merchant, store, expected, nil, calculate)
}

func CreateGuestFoodQuote(ctx context.Context, db *sql.DB, guest Principal, expected int64, calculate func(PublicOffering) (QuoteCalculation, error)) (FoodQuote, error) {
	return createFoodQuote(ctx, db, guest.MerchantID, guest.StoreID, expected, &guest, calculate)
}

func createFoodQuote(ctx context.Context, db *sql.DB, merchant, store uuid.UUID, expected int64, guest *Principal, calculate func(PublicOffering) (QuoteCalculation, error)) (FoodQuote, error) {
	q := FoodQuote{}
	tx, err := TenantTransaction(ctx, db, merchant)
	if err != nil {
		return q, err
	}
	defer tx.Rollback()
	var guestID any
	if guest != nil {
		if err := checkGuest(ctx, tx, *guest); err != nil {
			return q, err
		}
		guestID = guest.ID
	}
	p, err := publicOffering(ctx, tx, merchant, store, true)
	if err != nil {
		return q, err
	}
	if p.Generation != expected {
		return q, &PublicationConflict{p.Generation}
	}
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
	q = FoodQuote{ID: uuid.New(), MerchantID: merchant, StoreID: store, Generation: p.Generation, Revision: p.Revision, Model: p.Model, Currency: "EUR", Lines: calculation.Lines, Total: calculation.Total}
	err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&q.CreatedAt)
	if err != nil {
		return q, err
	}
	q.ExpiresAt = q.CreatedAt.Add(10 * time.Minute)
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
