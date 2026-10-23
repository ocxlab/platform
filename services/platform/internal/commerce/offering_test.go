package commerce

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/offering"
	"testing"
)

func TestOfferingRejectsBeforeStorage(t *testing.T) {
	merchant := uuid.New()
	p := persistence.Principal{ID: uuid.New(), MerchantID: merchant, Scopes: []string{"offering:write"}}
	for _, tc := range []struct {
		name  string
		p     persistence.Principal
		store uuid.UUID
		base  int64
		key   string
		want  error
	}{
		{"no_principal", persistence.Principal{}, uuid.New(), 0, "valid-request-0001", ErrForbidden},
		{"other_merchant", persistence.Principal{ID: uuid.New(), MerchantID: uuid.New(), Scopes: p.Scopes}, uuid.New(), 0, "valid-request-0001", ErrForbidden},
		{"provider", persistence.Principal{ID: p.ID, MerchantID: merchant, Scopes: p.Scopes, Provider: true}, uuid.New(), 0, "valid-request-0001", ErrForbidden},
		{"no_store", p, uuid.Nil, 0, "valid-request-0001", ErrInvalid},
		{"negative_revision", p, uuid.New(), -1, "valid-request-0001", ErrInvalid},
		{"short_key", p, uuid.New(), 0, "short", ErrInvalid},
		{"leading_punctuation", p, uuid.New(), 0, "_invalid-key-0001", ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (Offerings{}).Write(context.Background(), tc.p, merchant, tc.store, tc.base, offering.Model{}, nil, tc.key)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
