package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	foodstore "ocp.local/platform/internal/food/persistence"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
)

func (s *Server) CreateGuestSession(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	value, err := foodstore.CreateGuestSession(r.Context(), s.db, merchant, store)
	if err != nil {
		write(w, nil, err)
		return
	}
	write(w, persistence.Result{Status: 201, Body: mustJSON(value)}, nil)
}

func (s *Server) PlaceFoodOrder(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, params PlaceFoodOrderParams) {
	var body PlaceFoodOrderJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "A quote reference and pickup fulfilment are required.", nil)
		return
	}
	canonical, _ := json.Marshal(body)
	result, err := persistence.Mutate(r.Context(), s.db, principal(r), "placeFoodOrder", store.String(), params.IdempotencyKey, canonical, func(tx *sql.Tx) (any, error) {
		return foodstore.PlaceFoodOrder(r.Context(), tx, principal(r), body.QuoteId, uuid.MustParse(w.Header().Get("X-Request-ID")))
	})
	write(w, result, err)
}

func (s *Server) GetGuestFoodOrder(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID) {
	value, err := foodstore.GetGuestFoodOrder(r.Context(), s.db, principal(r), order)
	write(w, value, err)
}

func (s *Server) GetGuestFoodOrderByReference(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, reference string) {
	value, err := foodstore.GetGuestFoodOrderByReference(r.Context(), s.db, principal(r), reference)
	write(w, value, err)
}

func (s *Server) GetGuestFoodQuote(w http.ResponseWriter, r *http.Request, merchant, store, quote uuid.UUID) {
	value, err := foodstore.GetGuestFoodQuote(r.Context(), s.db, principal(r), quote)
	write(w, value, err)
}
