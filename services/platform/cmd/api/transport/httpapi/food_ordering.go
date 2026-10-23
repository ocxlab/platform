package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	foodstore "ocp.local/platform/internal/food/persistence"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
)

func (s *Server) GetFoodOrdering(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	value, err := foodstore.GetFoodOrdering(r.Context(), s.db, merchant, store, false)
	write(w, value, err)
}
func (s *Server) GetPublicFoodOrdering(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID) {
	value, err := foodstore.GetFoodOrdering(r.Context(), s.db, merchant, store, true)
	write(w, value, err)
}
func (s *Server) ChangeFoodOrdering(w http.ResponseWriter, r *http.Request, merchant, store uuid.UUID, params ChangeFoodOrderingParams) {
	var body ChangeFoodOrderingJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "Complete ordering settings are required.", nil)
		return
	}
	canonical, _ := json.Marshal(body)
	result, err := persistence.Mutate(r.Context(), s.db, principal(r), "changeFoodOrdering", store.String(), params.IdempotencyKey, canonical, func(tx *sql.Tx) (any, error) {
		return foodstore.ChangeFoodOrdering(r.Context(), tx, principal(r), store, body.ExpectedRevision, body.AcceptingOrders, string(body.AcceptanceMode), uuid.MustParse(w.Header().Get("X-Request-ID")))
	})
	write(w, result, err)
}
