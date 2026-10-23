package httpapi

import (
	"net/http"
	foodapp "ocp.local/platform/internal/food/application"
	foodstore "ocp.local/platform/internal/food/persistence"

	"github.com/google/uuid"
)

func (s *Server) ListMerchantFoodOrders(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, params ListMerchantFoodOrdersParams) {
	limit := 50
	if params.Limit != nil {
		limit = *params.Limit
	}
	value, err := (foodapp.Orders{DB: s.db}).List(r.Context(), principal(r), store, params.After, limit)
	write(w, value, err)
}
func (s *Server) GetMerchantFoodOrder(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID) {
	value, err := (foodapp.Orders{DB: s.db}).Get(r.Context(), principal(r), store, order)
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
	result, err := (foodapp.Orders{DB: s.db}).Change(r.Context(), principal(r), store, order, body.ExpectedRevision, string(body.Decision), params.IdempotencyKey, uuid.MustParse(w.Header().Get("X-Request-ID")))
	write(w, result, err)
}
func (s *Server) CancelGuestFoodOrder(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID, params CancelGuestFoodOrderParams) {
	var body CancelGuestFoodOrderJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "The current order revision is required.", nil)
		return
	}
	result, err := (foodapp.Checkout{DB: s.db}).Cancel(r.Context(), principal(r), order, body.ExpectedRevision, params.IdempotencyKey, uuid.MustParse(w.Header().Get("X-Request-ID")))
	write(w, result, err)
}

func (s *Server) ProgressFoodFulfilment(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID, params ProgressFoodFulfilmentParams) {
	var body ProgressFoodFulfilmentJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "A fulfilment action and current order revision are required.", nil)
		return
	}
	result, err := (foodapp.Orders{DB: s.db}).Change(r.Context(), principal(r), store, order, body.ExpectedRevision, string(body.Action), params.IdempotencyKey, uuid.MustParse(w.Header().Get("X-Request-ID")))
	write(w, result, err)
}
