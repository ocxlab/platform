package commerce

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"strings"
	"unicode/utf8"
)

type Accounts struct{ DB *sql.DB }

func (s Accounts) Inspect(ctx context.Context, p persistence.AccountPrincipal) (persistence.AccountProfile, error) {
	if !p.Allows("account:read") {
		return persistence.AccountProfile{}, ErrForbidden
	}
	return persistence.ReadAccount(ctx, s.DB, p)
}
func (s Accounts) Update(ctx context.Context, p persistence.AccountPrincipal, revision int64, name, locale, key string, request uuid.UUID) (persistence.Result, error) {
	if !p.Allows("account:read") || !p.Allows("account:write") {
		return persistence.Result{}, ErrForbidden
	}
	if revision < 1 || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 160 || strings.TrimSpace(name) == "" || strings.ContainsRune(name, 0) || (locale != "de" && locale != "en" && locale != "ar") || !ValidRequestKey(key) || request == uuid.Nil {
		return persistence.Result{}, ErrInvalid
	}
	return persistence.ChangeAccount(ctx, s.DB, p, revision, name, locale, key, request)
}
