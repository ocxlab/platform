package httpapi

import (
	"net/http"

	"github.com/google/uuid"
	"ocp.local/platform/internal/persistence"
)

func (s *Server) ListStores(w http.ResponseWriter, r *http.Request, merchant uuid.UUID, params ListStoresParams) {
	limit := 50
	if params.Limit != nil {
		limit = *params.Limit
	}
	page, err := persistence.ListStores(r.Context(), s.db, merchant, params.After, limit, params.Handle)
	write(w, page, err)
}
