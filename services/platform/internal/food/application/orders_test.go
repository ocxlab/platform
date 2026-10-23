package foodapp

import (
	"context"
	"errors"
	"github.com/google/uuid"
	commerce "ocp.local/platform/internal/core/application"
	"ocp.local/platform/internal/core/persistence"
	"testing"
)

func TestOrderAuthorizationBeforeDatabase(t *testing.T) {
	service := Orders{}
	ctx := context.Background()
	id := uuid.New()
	for _, p := range []persistence.Principal{{}, {ID: id, MerchantID: id, Scopes: []string{"offering:write"}}, {ID: id, MerchantID: id, Scopes: []string{"order:read", "order:manage"}, Provider: true}, {ID: id, MerchantID: id, Scopes: []string{"order:read", "order:manage"}, Guest: true}} {
		if _, err := service.List(ctx, p, id, nil, 20); !errors.Is(err, commerce.ErrForbidden) {
			t.Fatal(err)
		}
		if _, err := service.Get(ctx, p, id, id); !errors.Is(err, commerce.ErrForbidden) {
			t.Fatal(err)
		}
		if _, err := service.Change(ctx, p, id, id, 1, "accept", "test-request-key-1", id); !errors.Is(err, commerce.ErrForbidden) {
			t.Fatal(err)
		}
	}
	p := persistence.Principal{ID: id, MerchantID: id, Scopes: []string{"order:read", "order:manage"}}
	for _, limit := range []int{0, 101} {
		if _, err := service.List(ctx, p, id, nil, limit); !errors.Is(err, commerce.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := service.Get(ctx, p, id, uuid.Nil); !errors.Is(err, commerce.ErrInvalid) {
		t.Fatal(err)
	}
	for _, action := range []string{"cancel", "fake-payment", ""} {
		if _, err := service.Change(ctx, p, id, id, 1, action, "test-request-key-1", id); !errors.Is(err, commerce.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := service.Change(ctx, p, id, id, 1, "accept", "short", id); !errors.Is(err, commerce.ErrInvalid) {
		t.Fatal(err)
	}
}
