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
	"ocp.local/platform/internal/persistence/sqlgen"
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
	row, err := sqlgen.New(tx).CreateStore(ctx, sqlgen.CreateStoreParams{MerchantID: merchant, ID: store.ID, DisplayName: name})
	store.CreatedAt, store.Handle = row.CreatedAt, row.Handle
	return store, err
}

func GetStore(ctx context.Context, db *sql.DB, merchant, id uuid.UUID) (Store, error) {
	tx, err := TenantTransaction(ctx, db, merchant)
	if err != nil {
		return Store{}, err
	}
	defer tx.Rollback()
	row, err := sqlgen.New(tx).GetStore(ctx, sqlgen.GetStoreParams{MerchantID: merchant, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return Store{}, ErrNotFound
	}
	if err != nil {
		return Store{}, err
	}
	return Store{ID: row.ID, MerchantID: row.MerchantID, DisplayName: row.DisplayName, CreatedAt: row.CreatedAt, Handle: row.Handle}, tx.Commit()
}

func AppendOffering(ctx context.Context, tx *sql.Tx, merchant, store uuid.UUID, base int64, model offering.Model, document json.RawMessage) (Revision, error) {
	queries := sqlgen.New(tx)
	head, err := queries.LockOfferingHead(ctx, sqlgen.LockOfferingHeadParams{MerchantID: merchant, ID: store})
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
	revision.CreatedAt, err = queries.InsertOfferingRevision(ctx, sqlgen.InsertOfferingRevisionParams{MerchantID: merchant, StoreID: store, Revision: revision.Revision, ModelID: model.ID, ModelVersion: model.Version, Document: document})
	if err != nil {
		return Revision{}, err
	}
	err = queries.UpdateOfferingHead(ctx, sqlgen.UpdateOfferingHeadParams{MerchantID: merchant, ID: store, OfferingRevision: revision.Revision})
	return revision, err
}

func GetOffering(ctx context.Context, db *sql.DB, merchant, store uuid.UUID, revision int64) (Revision, error) {
	tx, err := TenantTransaction(ctx, db, merchant)
	if err != nil {
		return Revision{}, err
	}
	defer tx.Rollback()
	row, err := sqlgen.New(tx).GetOffering(ctx, sqlgen.GetOfferingParams{MerchantID: merchant, StoreID: store, RequestedRevision: revision})
	if errors.Is(err, sql.ErrNoRows) {
		return Revision{}, ErrNotFound
	}
	if err != nil {
		return Revision{}, err
	}
	return Revision{MerchantID: row.MerchantID, StoreID: row.StoreID, Revision: row.Revision, Model: offering.Model{ID: row.ModelID, Version: row.ModelVersion}, Document: row.Document, CreatedAt: row.CreatedAt, State: "draft"}, tx.Commit()
}
