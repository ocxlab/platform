package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	foodstore "ocp.local/platform/internal/food/persistence"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
)

func (s *Server) ListMerchantFoodOrders(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, params ListMerchantFoodOrdersParams) {
	limit := 50
	if params.Limit != nil {
		limit = *params.Limit
	}
	value, err := foodstore.ListFoodOrders(r.Context(), s.db, principal(r), store, params.After, limit)
	write(w, value, err)
}
func (s *Server) GetMerchantFoodOrder(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID) {
	value, err := foodstore.GetFoodOrder(r.Context(), s.db, principal(r), store, order)
	write(w, value, err)
}
func (s *Server) AdminGetFoodOrder(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID) {
	value, err := foodstore.GetFoodOrder(r.Context(), s.db, principal(r), store, order)
	write(w, value, err)
}
func (s *Server) DecideFoodOrder(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID, params DecideFoodOrderParams) {
	var body DecideFoodOrderJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "A decision and current order revision are required.", nil)
		return
	}
	canonical, _ := json.Marshal(body)
	result, err := persistence.Mutate(r.Context(), s.db, principal(r), "decideFoodOrder", store.String()+"/"+order.String(), params.IdempotencyKey, canonical, func(tx *sql.Tx) (any, error) {
		return foodstore.ChangeFoodOrder(r.Context(), tx, principal(r), store, order, body.ExpectedRevision, string(body.Decision), uuid.MustParse(w.Header().Get("X-Request-ID")))
	})
	write(w, result, err)
}
func (s *Server) CancelGuestFoodOrder(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID, params CancelGuestFoodOrderParams) {
	var body CancelGuestFoodOrderJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "The current order revision is required.", nil)
		return
	}
	canonical, _ := json.Marshal(body)
	result, err := persistence.Mutate(r.Context(), s.db, principal(r), "cancelGuestFoodOrder", store.String()+"/"+order.String(), params.IdempotencyKey, canonical, func(tx *sql.Tx) (any, error) {
		return foodstore.ChangeFoodOrder(r.Context(), tx, principal(r), store, order, body.ExpectedRevision, "cancel", uuid.MustParse(w.Header().Get("X-Request-ID")))
	})
	write(w, result, err)
}

func (s *Server) ProgressFoodFulfilment(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID, params ProgressFoodFulfilmentParams) {
	var body ProgressFoodFulfilmentJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "A fulfilment action and current order revision are required.", nil)
		return
	}
	canonical, _ := json.Marshal(body)
	result, err := persistence.Mutate(r.Context(), s.db, principal(r), "progressFoodFulfilment", store.String()+"/"+order.String(), params.IdempotencyKey, canonical, func(tx *sql.Tx) (any, error) {
		return foodstore.ProgressFoodFulfilment(r.Context(), tx, principal(r), store, order, body.ExpectedRevision, string(body.Action), uuid.MustParse(w.Header().Get("X-Request-ID")))
	})
	write(w, result, err)
}
