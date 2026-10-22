package commerce

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/google/uuid"
	"ocp.local/platform/internal/media"
	"ocp.local/platform/internal/persistence"
)

type MediaStorage interface {
	Upload(context.Context, string, string, string, int64, time.Duration) (*v4.PresignedHTTPRequest, error)
	Download(context.Context, string, time.Duration) (*v4.PresignedHTTPRequest, error)
	Process(context.Context, string, string, string, string, int64) (int, int, error)
}
type Media struct {
	DB      *sql.DB
	Storage MediaStorage
}

func (s Media) Start(ctx context.Context, p persistence.Principal, merchant, store uuid.UUID, kind, checksum string, size int64, key string) (persistence.Result, error) {
	if !p.AuthorizesMerchant(merchant, "offering:write") {
		return persistence.Result{}, ErrForbidden
	}
	sum, err := base64.StdEncoding.DecodeString(checksum)
	if store == uuid.Nil || !replayKey.MatchString(key) || err != nil || len(sum) != 32 || base64.StdEncoding.EncodeToString(sum) != checksum || size < 1 || size > media.MaxUploadBytes || (kind != "image/jpeg" && kind != "image/png" && kind != "image/webp") {
		return persistence.Result{}, ErrInvalid
	}
	canonical, _ := json.Marshal(struct {
		Type     string `json:"content_type"`
		Checksum string `json:"sha256"`
		Size     int64  `json:"size_bytes"`
	}{kind, checksum, size})
	return persistence.Mutate(ctx, s.DB, p, "startMediaUpload", store.String(), key, canonical, func(tx *sql.Tx) (any, error) { return persistence.CreateMedia(ctx, tx, p, store, kind, checksum, size) })
}
func (s Media) Read(ctx context.Context, p persistence.Principal, merchant, store, id uuid.UUID) (persistence.MediaAsset, error) {
	if !p.AuthorizesMerchant(merchant, "offering:read") {
		return persistence.MediaAsset{}, ErrForbidden
	}
	return persistence.GetMedia(ctx, s.DB, merchant, store, id)
}
func (s Media) Grant(ctx context.Context, p persistence.Principal, merchant, store, id uuid.UUID) (*v4.PresignedHTTPRequest, error) {
	if !p.AuthorizesMerchant(merchant, "offering:write") {
		return nil, ErrForbidden
	}
	a, err := persistence.GetMedia(ctx, s.DB, merchant, store, id)
	if err != nil {
		return nil, err
	}
	if a.State != "pending" || !time.Now().Before(a.ExpiresAt) {
		return nil, media.ErrInvalid
	}
	ttl := min(media.GrantLifetime, time.Until(a.ExpiresAt))
	if ttl < time.Second {
		return nil, media.ErrInvalid
	}
	return s.Storage.Upload(ctx, a.OriginalKey(), a.ContentType, a.Checksum, a.Size, ttl)
}
func (s Media) Complete(ctx context.Context, p persistence.Principal, merchant, store, id uuid.UUID, key string) (persistence.Result, error) {
	if !p.AuthorizesMerchant(merchant, "offering:write") {
		return persistence.Result{}, ErrForbidden
	}
	if !replayKey.MatchString(key) {
		return persistence.Result{}, ErrInvalid
	}
	a, err := persistence.GetMedia(ctx, s.DB, merchant, store, id)
	if err != nil {
		return persistence.Result{}, err
	}
	w, h := 0, 0
	if a.State != "ready" {
		if !time.Now().Before(a.ExpiresAt) {
			return persistence.Result{}, media.ErrInvalid
		}
		w, h, err = s.Storage.Process(ctx, a.OriginalKey(), a.ImageKey(), a.ContentType, a.Checksum, a.Size)
		if err != nil {
			return persistence.Result{}, err
		}
	} else {
		w, h = *a.Width, *a.Height
	}
	return persistence.Mutate(ctx, s.DB, p, "completeMediaUpload", store.String()+"/"+id.String(), key, []byte(`{}`), func(tx *sql.Tx) (any, error) { return persistence.CompleteMedia(ctx, tx, p, store, id, w, h) })
}
