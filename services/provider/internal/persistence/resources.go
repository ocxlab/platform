package persistence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/offering"
)

var ErrNotFound = errors.New("resource not found")
var ErrReplayConflict = errors.New("replay conflict")

type RevisionConflict struct{ Current int64 }

func (e *RevisionConflict) Error() string { return "revision conflict" }

type Store struct {
	Handle      string    `json:"handle,omitempty"`
	ID          uuid.UUID `json:"id"`
	MerchantID  uuid.UUID `json:"merchant_id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}
type Revision struct {
	MerchantID uuid.UUID       `json:"merchant_id"`
	StoreID    uuid.UUID       `json:"store_id"`
	Revision   int64           `json:"revision"`
	Model      offering.Model  `json:"model"`
	Document   json.RawMessage `json:"document"`
	CreatedAt  time.Time       `json:"created_at"`
	State      string          `json:"state"`
}
type Result struct {
	Status   int
	Body     json.RawMessage
	Replayed bool
}

// Mutate serializes a scoped replay key and commits its result with the mutation.
// Reconciliation of external effects is later purchase/payment work, not hidden here.
func Mutate(ctx context.Context, db *sql.DB, principal Principal, operation, resource, key string, canonical []byte, change func(*sql.Tx) (any, error)) (Result, error) {
	tx, err := TenantTransaction(ctx, db, principal.MerchantID)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if principal.Guest {
		if err := checkGuest(ctx, tx, principal); err != nil {
			return Result{}, err
		}
	}
	fingerprint, err := json.Marshal(struct {
		Operation string          `json:"operation"`
		Resource  string          `json:"resource"`
		Request   json.RawMessage `json:"request"`
	}{operation, resource, canonical})
	if err != nil {
		return Result{}, err
	}
	digest := sha256.Sum256(fingerprint)
	lock, _ := json.Marshal([]string{principal.MerchantID.String(), principal.ID.String(), operation, resource, key})
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, string(lock)); err != nil {
		return Result{}, err
	}
	var previous []byte
	var result Result
	table := "commerce.request_replays"
	if principal.Provider {
		table = "commerce.provider_request_replays"
	}
	if principal.Guest {
		table = "commerce.guest_request_replays"
	}
	err = tx.QueryRowContext(ctx, `SELECT request_hash,response_status,response_body
  FROM `+table+` WHERE merchant_id=$1 AND grant_id=$2 AND operation=$3 AND resource=$4 AND replay_key=$5`, principal.MerchantID, principal.ID, operation, resource, key).Scan(&previous, &result.Status, &result.Body)
	if err == nil {
		if !bytes.Equal(previous, digest[:]) {
			return Result{}, ErrReplayConflict
		}
		result.Replayed = true
		if err := tx.Commit(); err != nil {
			return Result{}, err
		}
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	value, err := change(tx)
	if err != nil {
		return Result{}, err
	}
	result.Status = 201
	result.Body, err = json.Marshal(value)
	if err != nil {
		return Result{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO `+table+`
 (merchant_id,grant_id,operation,resource,replay_key,request_hash,response_status,response_body)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, principal.MerchantID, principal.ID, operation, resource, key, digest[:], result.Status, []byte(result.Body)); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, err
	}
	return result, nil
}

func CreateStore(ctx context.Context, tx *sql.Tx, merchant uuid.UUID, name string) (Store, error) {
	store := Store{ID: uuid.New(), MerchantID: merchant, DisplayName: name}
	err := tx.QueryRowContext(ctx, `INSERT INTO commerce.stores(merchant_id,id,display_name)
  VALUES ($1,$2,$3) RETURNING created_at,'store-'||display_number`, merchant, store.ID, name).Scan(&store.CreatedAt, &store.Handle)
	return store, err
}

func GetStore(ctx context.Context, db *sql.DB, merchant, id uuid.UUID) (Store, error) {
	tx, err := TenantTransaction(ctx, db, merchant)
	if err != nil {
		return Store{}, err
	}
	defer tx.Rollback()
	var store Store
	err = tx.QueryRowContext(ctx, `SELECT id,merchant_id,display_name,created_at,'store-'||display_number FROM commerce.stores WHERE merchant_id=$1 AND id=$2`, merchant, id).Scan(&store.ID, &store.MerchantID, &store.DisplayName, &store.CreatedAt, &store.Handle)
	if errors.Is(err, sql.ErrNoRows) {
		return Store{}, ErrNotFound
	}
	if err != nil {
		return Store{}, err
	}
	return store, tx.Commit()
}

func AppendOffering(ctx context.Context, tx *sql.Tx, merchant, store uuid.UUID, base int64, model offering.Model, document json.RawMessage) (Revision, error) {
	var head int64
	err := tx.QueryRowContext(ctx, `SELECT offering_revision FROM commerce.stores WHERE merchant_id=$1 AND id=$2 FOR UPDATE`, merchant, store).Scan(&head)
	if errors.Is(err, sql.ErrNoRows) {
		return Revision{}, ErrNotFound
	}
	if err != nil {
		return Revision{}, err
	}
	if head != base {
		return Revision{}, &RevisionConflict{Current: head}
	}
	if err := validateMediaReferences(ctx, tx, merchant, store, model, document); err != nil {
		return Revision{}, err
	}
	revision := Revision{MerchantID: merchant, StoreID: store, Revision: head + 1, Model: model, Document: document, State: "draft"}
	err = tx.QueryRowContext(ctx, `INSERT INTO commerce.offering_revisions(merchant_id,store_id,revision,model_id,model_version,document)
 VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at`, merchant, store, revision.Revision, model.ID, model.Version, string(document)).Scan(&revision.CreatedAt)
	if err != nil {
		return Revision{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE commerce.stores SET offering_revision=$3 WHERE merchant_id=$1 AND id=$2`, merchant, store, revision.Revision)
	return revision, err
}

func GetOffering(ctx context.Context, db *sql.DB, merchant, store uuid.UUID, revision int64) (Revision, error) {
	tx, err := TenantTransaction(ctx, db, merchant)
	if err != nil {
		return Revision{}, err
	}
	defer tx.Rollback()
	var result Revision
	result.State = "draft"
	err = tx.QueryRowContext(ctx, `SELECT r.merchant_id,r.store_id,r.revision,r.model_id,r.model_version,r.document,r.created_at
 FROM commerce.offering_revisions r JOIN commerce.stores s ON (r.merchant_id,r.store_id)=(s.merchant_id,s.id)
 WHERE r.merchant_id=$1 AND r.store_id=$2 AND r.revision=CASE WHEN $3::bigint=0 THEN s.offering_revision ELSE $3::bigint END`, merchant, store, revision).Scan(&result.MerchantID, &result.StoreID, &result.Revision, &result.Model.ID, &result.Model.Version, &result.Document, &result.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Revision{}, ErrNotFound
	}
	if err != nil {
		return Revision{}, err
	}
	return result, tx.Commit()
}
