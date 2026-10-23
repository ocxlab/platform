package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"ocp.local/platform/internal/commerce"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/infra/media"
)

func (s *Server) mediaEnabled(w http.ResponseWriter) bool {
	if s.mediaStore == nil {
		problem(w, 503, "unavailable", "Image storage is not configured.", nil)
		return false
	}
	return true
}
func mediaResult(w http.ResponseWriter, value any, err error) {
	switch {
	case errors.Is(err, commerce.ErrForbidden):
		problem(w, 403, "forbidden", "The credential does not authorize this image action.", nil)
	case errors.Is(err, commerce.ErrInvalid):
		problem(w, 400, "invalid_request", "Invalid media upload request.", nil)
	case errors.Is(err, media.ErrInvalid):
		problem(w, 422, "invalid_media", "The file is invalid, does not match its declaration, or the pending upload has expired.", nil)
	case errors.Is(err, persistence.ErrMediaCapacity):
		problem(w, 429, "media_capacity", "The store has reached its image allocation limit.", nil)
	default:
		write(w, value, err)
	}
}
func (s *Server) StartMediaUpload(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, params StartMediaUploadParams) {
	if !s.mediaEnabled(w) {
		return
	}
	var body StartMediaUploadJSONRequestBody
	if err := decode(r, &body); err != nil {
		problem(w, 400, "invalid_request", "Invalid upload declaration.", nil)
		return
	}
	result, err := (commerce.Media{DB: s.db, Storage: s.mediaStore}).Start(r.Context(), principal(r), merchant, store, string(body.ContentType), body.Sha256, body.SizeBytes, params.IdempotencyKey)
	mediaResult(w, result, err)
}
func (s *Server) GetMediaAsset(w http.ResponseWriter, r *http.Request, merchant, store, id uuid.UUID) {
	if !s.mediaEnabled(w) {
		return
	}
	a, err := (commerce.Media{DB: s.db, Storage: s.mediaStore}).Read(r.Context(), principal(r), merchant, store, id)
	mediaResult(w, a, err)
}
func (s *Server) CreateMediaUploadGrant(w http.ResponseWriter, r *http.Request, merchant, store, id uuid.UUID) {
	if !s.mediaEnabled(w) {
		return
	}
	grant, err := (commerce.Media{DB: s.db, Storage: s.mediaStore}).Grant(r.Context(), principal(r), merchant, store, id)
	if err != nil {
		mediaResult(w, nil, err)
		return
	}
	headers := map[string]string{}
	for name, values := range grant.SignedHeader {
		if !strings.EqualFold(name, "Host") && !strings.EqualFold(name, "Content-Length") {
			headers[name] = strings.Join(values, ",")
		}
	}
	parsed, _ := url.Parse(grant.URL)
	seconds, _ := strconv.Atoi(parsed.Query().Get("X-Amz-Expires"))
	write(w, map[string]any{"url": grant.URL, "method": "PUT", "headers": headers, "expires_in_seconds": seconds}, nil)
}
func (s *Server) CompleteMediaUpload(w http.ResponseWriter, r *http.Request, merchant, store, id uuid.UUID, params CompleteMediaUploadParams) {
	if !s.mediaEnabled(w) {
		return
	}
	result, err := (commerce.Media{DB: s.db, Storage: s.mediaStore}).Complete(r.Context(), principal(r), merchant, store, id, params.IdempotencyKey)
	mediaResult(w, result, err)
}
func (s *Server) GetMerchantMediaImage(w http.ResponseWriter, r *http.Request, merchant, store, id uuid.UUID) {
	if !s.mediaEnabled(w) {
		return
	}
	a, err := (commerce.Media{DB: s.db, Storage: s.mediaStore}).Read(r.Context(), principal(r), merchant, store, id)
	if err == nil && a.State != "ready" {
		err = persistence.ErrNotFound
	}
	if err != nil {
		mediaResult(w, nil, err)
		return
	}
	signed, err := s.mediaStore.Download(r.Context(), a.ImageKey(), media.GrantLifetime)
	if err != nil {
		mediaResult(w, nil, err)
		return
	}
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, signed.URL, http.StatusTemporaryRedirect)
}
func (s *Server) GetPublicMediaImage(w http.ResponseWriter, r *http.Request, merchant, store, id uuid.UUID) {
	if !s.mediaEnabled(w) {
		return
	}
	a, err := persistence.PublicMedia(r.Context(), s.db, merchant, store, id)
	if err != nil {
		mediaResult(w, nil, err)
		return
	}
	signed, err := s.mediaStore.Download(r.Context(), a.ImageKey(), media.GrantLifetime)
	if err != nil {
		mediaResult(w, nil, err)
		return
	}
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, signed.URL, http.StatusTemporaryRedirect)
}
