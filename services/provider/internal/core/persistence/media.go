package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"ocp.local/platform/internal/database"
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
	err := tx.QueryRowContext(ctx, `SELECT content_type,size_bytes,checksum,state,width,height,created_at,expires_at FROM commerce.media_assets WHERE merchant_id=$1 AND store_id=$2 AND id=$3`, merchant, store, id).Scan(&a.ContentType, &a.Size, &a.Checksum, &a.State, &a.Width, &a.Height, &a.CreatedAt, &a.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
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
	var count int
	var allocated int64
	if err := tx.QueryRowContext(ctx, `SELECT count(*),coalesce(sum(size_bytes),0) FROM commerce.media_assets WHERE merchant_id=$1 AND store_id=$2`, p.MerchantID, store).Scan(&count, &allocated); err != nil {
		return MediaAsset{}, err
	}
	// Explicit bounded development policy. No silent reclamation of referenced
	// history; lifecycle cleanup requires a separate retention policy.
	if count >= 500 || allocated+size > 200*1024*1024 {
		return MediaAsset{}, ErrMediaCapacity
	}
	id := uuid.New()
	_, err := tx.ExecContext(ctx, `INSERT INTO commerce.media_assets(merchant_id,store_id,id,content_type,size_bytes,checksum,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, p.MerchantID, store, id, kind, size, checksum, p.ID)
	if err != nil {
		return MediaAsset{}, err
	}
	return mediaAsset(ctx, tx, p.MerchantID, store, id)
}
func CompleteMedia(ctx context.Context, tx *sql.Tx, p Principal, store, id uuid.UUID, w, h int) (MediaAsset, error) {
	_, err := tx.ExecContext(ctx, `UPDATE commerce.media_assets SET state='ready',width=$4,height=$5,completed_at=coalesce(completed_at,clock_timestamp()) WHERE merchant_id=$1 AND store_id=$2 AND id=$3`, p.MerchantID, store, id, w, h)
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
