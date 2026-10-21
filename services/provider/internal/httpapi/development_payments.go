package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"ocp.local/platform/internal/persistence"
)

func (s *Server) ChangeDevelopmentPayment(w http.ResponseWriter, r *http.Request, merchant, store, order uuid.UUID, params ChangeDevelopmentPaymentParams) {
	if !s.developmentPayments {
		problem(w, 503, "development_payments_disabled", "Development payments are disabled.", nil)
		return
	}
	var body ChangeDevelopmentPaymentJSONRequestBody
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "A simulation action and current order revision are required.", nil)
		return
	}
	canonical, _ := json.Marshal(body)
	result, err := persistence.Mutate(r.Context(), s.db, principal(r), "changeDevelopmentPayment", store.String()+"/"+order.String(), params.IdempotencyKey, canonical, func(tx *sql.Tx) (any, error) {
		return persistence.ChangeDevelopmentPayment(r.Context(), tx, principal(r), store, order, body.ExpectedRevision, string(body.Action), uuid.MustParse(w.Header().Get("X-Request-ID")))
	})
	write(w, result, err)
}
