package httpapi

import (
	"errors"
	"net/http"

	"ocp.local/platform/internal/core/persistence"
)

func (s *Server) adminDirectory(w http.ResponseWriter, r *http.Request, kind, q, status string, after *string, limit *int) {
	size := 25
	if limit != nil {
		size = *limit
	}
	page, err := persistence.ListAdminRecords(r.Context(), s.db, principal(r), kind, q, status, after, size)
	if errors.Is(err, persistence.ErrAdminCursor) {
		problem(w, 400, "invalid_request", "Restart the search when changing filters or using an invalid continuation.", nil)
		return
	}
	write(w, page, err)
}
func (s *Server) AdminListStores(w http.ResponseWriter, r *http.Request, params AdminListStoresParams) {
	q, status := "", ""
	if params.Q != nil {
		q = *params.Q
	}
	if params.Status != nil {
		status = string(*params.Status)
	}
	s.adminDirectory(w, r, "stores", q, status, params.After, params.Limit)
}
func (s *Server) AdminListFoodOrders(w http.ResponseWriter, r *http.Request, params AdminListFoodOrdersParams) {
	q, status := "", ""
	if params.Q != nil {
		q = *params.Q
	}
	if params.Status != nil {
		status = string(*params.Status)
	}
	s.adminDirectory(w, r, "orders", q, status, params.After, params.Limit)
}
