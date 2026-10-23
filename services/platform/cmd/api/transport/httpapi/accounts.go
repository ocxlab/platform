package httpapi

import (
	"errors"
	"github.com/google/uuid"
	"net/http"
	commerce "ocp.local/platform/internal/core/application"
	"ocp.local/platform/internal/core/persistence"
)

type accountPrincipalKey struct{}

func (s *Server) GetMerchantAccountProfile(w http.ResponseWriter, r *http.Request) {
	s.getAccount(w, r)
}
func (s *Server) GetCustomerAccountProfile(w http.ResponseWriter, r *http.Request) {
	s.getAccount(w, r)
}
func (s *Server) ChangeMerchantAccountProfile(w http.ResponseWriter, r *http.Request, p ChangeMerchantAccountProfileParams) {
	s.changeAccount(w, r, p.IdempotencyKey)
}
func (s *Server) ChangeCustomerAccountProfile(w http.ResponseWriter, r *http.Request, p ChangeCustomerAccountProfileParams) {
	s.changeAccount(w, r, p.IdempotencyKey)
}
func (s *Server) getAccount(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(accountPrincipalKey{}).(persistence.AccountPrincipal)
	value, err := (commerce.Accounts{DB: s.db}).Inspect(r.Context(), p)
	writeAccount(w, value, err)
}
func (s *Server) changeAccount(w http.ResponseWriter, r *http.Request, key string) {
	var body ChangeAccountProfile
	if decode(r, &body) != nil {
		problem(w, 400, "invalid_request", "Editable preferences are required.", nil)
		return
	}
	p := r.Context().Value(accountPrincipalKey{}).(persistence.AccountPrincipal)
	value, err := (commerce.Accounts{DB: s.db}).Update(r.Context(), p, body.ExpectedRevision, body.DisplayName, string(body.Locale), key, uuid.MustParse(w.Header().Get("X-Request-ID")))
	writeAccount(w, value, err)
}
func writeAccount(w http.ResponseWriter, value any, err error) {
	var conflict *persistence.AccountRevisionConflict
	switch {
	case errors.As(err, &conflict):
		problem(w, 409, "account_revision_conflict", "Profile changed. Inspect its current revision before editing.", &conflict.Current)
	case errors.Is(err, commerce.ErrForbidden):
		problem(w, 403, "forbidden", "Account scope is required.", nil)
	case errors.Is(err, commerce.ErrInvalid):
		problem(w, 400, "invalid_request", "Check profile fields, revision, and retry key.", nil)
	default:
		write(w, value, err)
	}
}
