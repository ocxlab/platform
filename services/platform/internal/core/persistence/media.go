package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"ocp.local/platform/internal/core/persistence/sqlgen"
	"ocp.local/platform/internal/infra/database"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/offering"
)

var ErrMediaCapacity = errors.New("media storage allocation limit reached")
var ErrMediaReference = errors.New("media reference is not ready or owned by this store")

type MediaAsset struct {
	ID          uuid.UUID `json:"id"`
	MerchantID  uuid.UUID `json:"merchant_id"`
	StoreID     uuid.UUID `json:"store_id"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size_bytes"`
	Checksum    string    `json:"sha256"`
	State       string    `json:"state"`
	Width       *int      `json:"width,omitempty"`
	Height      *int      `json:"height,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (a MediaAsset) OriginalKey() string {
	return "uploads/" + a.MerchantID.String() + "/" + a.StoreID.String() + "/" + a.ID.String()
}
func (a MediaAsset) ImageKey() string {
	return "images/" + a.MerchantID.String() + "/" + a.StoreID.String() + "/" + a.ID.String() + ".jpg"
}

func mediaAsset(ctx context.Context, tx *sql.Tx, merchant, store, id uuid.UUID) (MediaAsset, error) {
	a := MediaAsset{ID: id, MerchantID: merchant, StoreID: store}
	row, err := sqlgen.New(tx).GetMediaAsset(ctx, sqlgen.GetMediaAssetParams{MerchantID: merchant, StoreID: store, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	a.ContentType, a.Size, a.Checksum, a.State = row.ContentType, row.SizeBytes, row.Checksum, row.State
	a.CreatedAt, a.ExpiresAt = row.CreatedAt, row.ExpiresAt
	if row.Width.Valid {
		width := int(row.Width.Int32)
		a.Width = &width
	}
	if row.Height.Valid {
		height := int(row.Height.Int32)
		a.Height = &height
	}
	return a, err
}
func GetMedia(ctx context.Context, db *sql.DB, merchant, store, id uuid.UUID) (MediaAsset, error) {
	tx, err := database.TenantTransaction(ctx, db, merchant)
	if err != nil {
		return MediaAsset{}, err
	}
	defer tx.Rollback()
	a, err := mediaAsset(ctx, tx, merchant, store, id)
	if err != nil {
		return a, err
	}
	return a, tx.Commit()
}
func CreateMedia(ctx context.Context, tx *sql.Tx, p Principal, store uuid.UUID, kind, checksum string, size int64) (MediaAsset, error) {
	// Serialize allocations with each other and offering edits on the store row.
	if _, err := publication(ctx, tx, p.MerchantID, store, true); err != nil {
		return MediaAsset{}, err
	}
	queries := sqlgen.New(tx)
	allocation, err := queries.GetMediaAllocation(ctx, sqlgen.GetMediaAllocationParams{MerchantID: p.MerchantID, StoreID: store})
	if err != nil {
		return MediaAsset{}, err
	}
	// Explicit bounded development policy. No silent reclamation of referenced
	// history; lifecycle cleanup requires a separate retention policy.
	if allocation.Assets >= 500 || allocation.Allocated+size > 200*1024*1024 {
		return MediaAsset{}, ErrMediaCapacity
	}
	id := uuid.New()
	err = queries.InsertMediaAsset(ctx, sqlgen.InsertMediaAssetParams{MerchantID: p.MerchantID, StoreID: store, ID: id, ContentType: kind, SizeBytes: size, Checksum: checksum, ActorID: p.ID})
	if err != nil {
		return MediaAsset{}, err
	}
	return mediaAsset(ctx, tx, p.MerchantID, store, id)
}
func CompleteMedia(ctx context.Context, tx *sql.Tx, p Principal, store, id uuid.UUID, w, h int) (MediaAsset, error) {
	err := sqlgen.New(tx).CompleteMediaAsset(ctx, sqlgen.CompleteMediaAssetParams{MerchantID: p.MerchantID, StoreID: store, ID: id, Width: int64(w), Height: int64(h)})
	if err != nil {
		return MediaAsset{}, err
	}
	return mediaAsset(ctx, tx, p.MerchantID, store, id)
}

// Model-owned references are checked inside the same transaction as a new
// immutable revision. Copying a foreign or incomplete ID cannot publish it.
func validateMediaReferences(ctx context.Context, tx *sql.Tx, merchant, store uuid.UUID, model offering.Model, raw json.RawMessage) error {
	if model.ID != "ocp.food.menu" {
		return nil
	}
	var doc struct {
		Items []struct {
			ImageID string `json:"image_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, item := range doc.Items {
		if item.ImageID == "" || seen[item.ImageID] {
			continue
		}
		seen[item.ImageID] = true
		id, err := uuid.Parse(item.ImageID)
		if err != nil {
			return ErrMediaReference
		}
		a, err := mediaAsset(ctx, tx, merchant, store, id)
		if errors.Is(err, ErrNotFound) {
			return ErrMediaReference
		}
		if err != nil {
			return err
		}
		if a.State != "ready" {
			return ErrMediaReference
		}
	}
	return nil
}

func PublicMedia(ctx context.Context, db *sql.DB, merchant, store, id uuid.UUID) (MediaAsset, error) {
	tx, err := database.TenantTransaction(ctx, db, merchant)
	if err != nil {
		return MediaAsset{}, err
	}
	defer tx.Rollback()
	// Authorization is the visible model projection, not knowledge of a UUID.
	public, err := PublicOfferingTx(ctx, tx, merchant, store, false)
	if err != nil {
		return MediaAsset{}, err
	}
	if public.Model.ID != "ocp.food.menu" {
		return MediaAsset{}, ErrNotFound
	}
	var doc struct {
		Items []struct {
			ImageID string `json:"image_id"`
		} `json:"items"`
	}
	if err = json.Unmarshal(public.Document, &doc); err != nil {
		return MediaAsset{}, err
	}
	found := false
	for _, item := range doc.Items {
		if item.ImageID == id.String() {
			found = true
			break
		}
	}
	if !found {
		return MediaAsset{}, ErrNotFound
	}
	a, err := mediaAsset(ctx, tx, merchant, store, id)
	if err != nil {
		return a, err
	}
	if a.State != "ready" {
		return a, ErrNotFound
	}
	return a, tx.Commit()
}
