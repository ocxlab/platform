package commerce

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"strings"
	"testing"
)

func TestAccountInputBoundary(t *testing.T) {
	app := Accounts{}
	p := persistence.AccountPrincipal{ID: uuid.New(), AccountID: uuid.New(), Kind: "customer", Resource: "customer-account-mcp", Scopes: []string{"account:read", "account:write"}}
	for _, tc := range []struct {
		rev               int64
		name, locale, key string
	}{
		{0, "Example", "en", "account-request-0001"}, {1, " ", "en", "account-request-0001"},
		{1, strings.Repeat("a", 161), "en", "account-request-0001"}, {1, "bad\x00name", "en", "account-request-0001"},
		{1, string([]byte{255}), "en", "account-request-0001"}, {1, "Example", "fr", "account-request-0001"}, {1, "Example", "en", "short"},
	} {
		if _, err := app.Update(context.Background(), p, tc.rev, tc.name, tc.locale, tc.key, uuid.New()); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
	p.Scopes = []string{"account:read"}
	if _, err := app.Update(context.Background(), p, 1, "Example", "ar", "account-request-0001", uuid.New()); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	p.Scopes = nil
	if _, err := app.Inspect(context.Background(), p); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
}
