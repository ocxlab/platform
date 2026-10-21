package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"ocp.local/platform/internal/offering/food"
	"ocp.local/platform/internal/persistence"
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
	canonical, _ := json.Marshal(body)
	result, err := persistence.Mutate(r.Context(), s.db, principal(r), "changePublication", store.String(), params.IdempotencyKey, canonical, func(tx *sql.Tx) (any, error) {
		return persistence.ChangePublication(r.Context(), tx, principal(r), store, body.ExpectedGeneration, body.Revision, nil, "", uuid.MustParse(w.Header().Get("X-Request-ID")), s.models.Project)
	})
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
	value, err := persistence.CreateFoodQuote(r.Context(), s.db, merchant, store, body.PublicationGeneration, func(p persistence.PublicOffering) (persistence.QuoteCalculation, error) {
		if p.Model != food.Model {
			return persistence.QuoteCalculation{}, food.ErrSelection
		}
		priced, total, err := food.Price(p.Document, lines)
		if err != nil {
			return persistence.QuoteCalculation{}, err
		}
		encoded, err := json.Marshal(priced)
		return persistence.QuoteCalculation{Lines: encoded, Total: total}, err
	})
	if err != nil {
		write(w, nil, err)
		return
	}
	write(w, persistence.Result{Status: 201, Body: mustJSON(value)}, nil)
}
func mustJSON(value any) json.RawMessage { data, _ := json.Marshal(value); return data }
