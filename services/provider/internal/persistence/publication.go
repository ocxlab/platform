package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/offering"
	"ocp.local/platform/internal/persistence/sqlgen"
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
	queries := sqlgen.New(tx)
	var revision sql.NullInt64
	var err error
	if lock {
		var row sqlgen.LockPublicationRow
		row, err = queries.LockPublication(ctx, sqlgen.LockPublicationParams{MerchantID: merchant, ID: store})
		p.Generation, revision, p.Suspended = row.PublicationGeneration, row.PublishedRevision, row.PublicationSuspended
	} else {
		var row sqlgen.ReadPublicationRow
		row, err = queries.ReadPublication(ctx, sqlgen.ReadPublicationParams{MerchantID: merchant, ID: store})
		p.Generation, revision, p.Suspended = row.PublicationGeneration, row.PublishedRevision, row.PublicationSuspended
	}
	if revision.Valid {
		p.Revision = &revision.Int64
	}
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
	previousRevision := p.Revision
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
		err = sqlgen.New(tx).SetPublicationSuspended(ctx, sqlgen.SetPublicationSuspendedParams{MerchantID: actor.MerchantID, ID: store, PublicationSuspended: p.Suspended})
	} else {
		if actor.Provider {
			return p, errors.New("merchant principal required")
		}
		var projected *json.RawMessage
		var publishedRevision sql.NullInt64
		if revision != nil {
			row, readErr := sqlgen.New(tx).GetPublicationSource(ctx, sqlgen.GetPublicationSourceParams{MerchantID: actor.MerchantID, StoreID: store, Revision: *revision})
			err = readErr
			if errors.Is(err, sql.ErrNoRows) {
				return p, ErrNotFound
			}
			if err != nil {
				return p, err
			}
			public, err := project(offering.Model{ID: row.ModelID, Version: row.ModelVersion}, row.Document)
			if err != nil {
				return p, err
			}
			projected = &public
			publishedRevision = sql.NullInt64{Int64: *revision, Valid: true}
			action = "publish"
		}
		p.Revision = revision
		err = sqlgen.New(tx).UpdatePublishedOffering(ctx, sqlgen.UpdatePublishedOfferingParams{MerchantID: actor.MerchantID, StoreID: store, Revision: publishedRevision, Document: projected})
	}
	if err != nil {
		return p, err
	}
	p.Generation++
	var auditRevision sql.NullInt64
	if p.Revision != nil {
		auditRevision = sql.NullInt64{Int64: *p.Revision, Valid: true}
	}
	err = sqlgen.New(tx).InsertPublicationAudit(ctx, sqlgen.InsertPublicationAuditParams{MerchantID: actor.MerchantID, StoreID: store, Generation: p.Generation, ActorID: actor.ID, ActorKind: kind, Action: action, Reason: reason, Revision: auditRevision, Suspended: p.Suspended, RequestID: requestID})
	if err != nil {
		return p, err
	}
	modelRevision := p.Revision
	if modelRevision == nil {
		modelRevision = previousRevision
	}
	if modelRevision != nil {
		row, err := sqlgen.New(tx).GetCatalogSnapshot(ctx, sqlgen.GetCatalogSnapshotParams{MerchantID: actor.MerchantID, StoreID: store, Revision: *modelRevision})
		if err != nil {
			return p, err
		}
		model := offering.Model{ID: row.ModelID, Version: row.ModelVersion}
		public := row.Document
		var domain capability.Domain
		switch model.ID {
		case "ocp.food.menu":
			domain = capability.Food
		case "ocp.shopping.catalog":
			domain = capability.Shopping
		case "ocp.lodging.property":
			domain = capability.Lodging
		default:
			return p, offering.ErrUnsupported
		}
		visible := p.Revision != nil && !p.Suspended
		if !visible {
			public = json.RawMessage(`null`)
		}
		payload, marshalErr := json.Marshal(CatalogIntent{Action: action, Visible: visible, Generation: p.Generation, Revision: modelRevision, Model: model, DisplayName: row.DisplayName, Document: public})
		if marshalErr != nil {
			return p, marshalErr
		}
		if _, err = EnqueueEffect(ctx, tx, Effect{MerchantID: actor.MerchantID, StoreID: store, Domain: domain, Kind: "catalog.publish", SubjectID: store, Key: fmt.Sprint(p.Generation), Payload: payload}); err != nil {
			return p, err
		}
	}
	return p, err
}

// CatalogIntent is a public-only, immutable snapshot made in the visibility
// transaction. Delivery never needs to read another tenant's mutable store.
// Action describes the actor's change; Visible is the resulting public state.
// A merchant can publish while provider-suspended, producing a hidden publish.
type CatalogIntent struct {
	Action      string          `json:"action"`
	Visible     bool            `json:"visible"`
	Generation  int64           `json:"generation"`
	Revision    *int64          `json:"revision"`
	Model       offering.Model  `json:"model"`
	DisplayName string          `json:"display_name"`
	Document    json.RawMessage `json:"document"`
}

type PublicOffering struct {
	EvaluatedAt time.Time       `json:"-"`
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
	ordering, err := foodOrdering(ctx, tx, merchant, store, false)
	if err != nil {
		return q, err
	}
	if !ordering.AcceptingOrders {
		return q, ErrOrderingPaused
	}
	if p.Generation != expected {
		return q, &PublicationConflict{p.Generation}
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
