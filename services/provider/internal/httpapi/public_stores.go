package httpapi

import (
	"net/http"
	"ocp.local/platform/internal/persistence"
)

func (s *Server) ListPublicStores(w http.ResponseWriter, r *http.Request, params ListPublicStoresParams) {
	limit := 50
	if params.Limit != nil {
		limit = *params.Limit
	}
	query := ""
	if params.Q != nil {
		query = *params.Q
	}
	value, err := persistence.ListPublicStores(r.Context(), s.db, query, params.After, limit)
	write(w, value, err)
}

func (s *Server) GetPublicStoreByHandle(w http.ResponseWriter, r *http.Request, handle string) {
	value, err := persistence.GetPublicStoreByHandle(r.Context(), s.db, handle)
	write(w, value, err)
}
