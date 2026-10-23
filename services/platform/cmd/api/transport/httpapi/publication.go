package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	foodapp "ocp.local/platform/internal/food/application"
	"strings"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/application"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/food/offering"
)

func (s *Server) GetPublication(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	value, err := persistence.GetPublication(r.Context(), s.db, merchant, store)
	write(w, value, err)
}
func (s *Server) AdminGetPublication(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	value, err := persistence.GetPublication(r.Context(), s.db, merchant, store)
	write(w, value, err)
}
func (s *Server) ChangePublication(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, params ChangePublicationParams) {
	var body ChangePublicationJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "A publication command is required.", nil)
		return
	}
	result, err := (commerce.Publications{DB: s.db, Models: s.models}).Change(r.Context(), principal(r), merchant, store, body.ExpectedGeneration, body.Revision, params.IdempotencyKey, uuid.MustParse(w.Header().Get("X-Request-ID")))
	write(w, result, err)
}
func (s *Server) AdminSuspendPublication(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, params AdminSuspendPublicationParams) {
	var body AdminSuspendPublicationJSONRequestBody
	if decode(r, &body) != nil || strings.TrimSpace(body.Reason) == "" {
		problem(w, 400, "invalid_request", "A nonblank administrative reason is required.", nil)
		return
	}
	canonical, _ := json.Marshal(body)
	result, err := persistence.Mutate(r.Context(), s.db, principal(r), "suspendPublication", store.String(), params.IdempotencyKey, canonical, func(tx *sql.Tx) (any, error) {
		return persistence.ChangePublication(r.Context(), tx, principal(r), store, body.ExpectedGeneration, nil, &body.Suspended, body.Reason, uuid.MustParse(w.Header().Get("X-Request-ID")), s.models.Project)
	})
	write(w, result, err)
}
func (s *Server) GetPublicOffering(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	value, err := persistence.GetPublicOffering(r.Context(), s.db, merchant, store)
	write(w, value, err)
}
func (s *Server) CreateFoodQuote(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	s.createFoodQuote(w, r, merchant, store, false)
}
func (s *Server) CreateGuestFoodQuote(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	s.createFoodQuote(w, r, merchant, store, true)
}
func (s *Server) createFoodQuote(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, guest bool) {
	var body CreateFoodQuoteJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "A complete food selection is required.", nil)
		return
	}
	data, _ := json.Marshal(body.Lines)
	var lines []food.SelectionLine
	if json.Unmarshal(data, &lines) != nil {
		problem(w, 400, "invalid_request", "Invalid food selection.", nil)
		return
	}
	locale, note := "", ""
	if body.Locale != nil {
		locale = string(*body.Locale)
	}
	if body.Note != nil {
		note = *body.Note
	}
	var actor persistence.Principal
	if guest {
		actor = principal(r)
	}
	value, err := (foodapp.Checkout{DB: s.db}).Quote(r.Context(), actor, merchant, store, body.PublicationGeneration, lines, locale, note, guest)
	if err != nil {
		write(w, nil, err)
		return
	}
	write(w, persistence.Result{Status: 201, Body: mustJSON(value)}, nil)
}
func mustJSON(value any) json.RawMessage { data, _ := json.Marshal(value); return data }
