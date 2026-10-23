package foodapp

import (
	"context"
	"errors"
	"github.com/google/uuid"
	commerce "ocp.local/platform/internal/core/application"
	core "ocp.local/platform/internal/core/persistence"
	food "ocp.local/platform/internal/food/offering"
	"strings"
	"testing"
)

func TestCheckoutRejectsBeforePersistence(t *testing.T) {
	s := Checkout{}
	ctx := context.Background()
	merchant, shop := uuid.New(), uuid.New()
	p := core.Principal{ID: uuid.New(), MerchantID: merchant, StoreID: shop, Guest: true, Scopes: []string{"quote:create", "order:create", "order:cancel"}}
	for _, note := range []string{strings.Repeat("ع", 501), "bad\x00note", string([]byte{0xff})} {
		if _, err := s.Quote(ctx, p, merchant, shop, 1, []food.SelectionLine{}, "en", note, true); !errors.Is(err, commerce.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := s.Quote(ctx, p, merchant, shop, 1, nil, "fr", "", true); !errors.Is(err, commerce.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Quote(ctx, p, merchant, uuid.New(), 1, nil, "", "", true); !errors.Is(err, commerce.ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := s.Place(ctx, core.Principal{}, uuid.New(), "valid-request-key-001", uuid.New()); !errors.Is(err, commerce.ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := s.Place(ctx, p, uuid.New(), "short", uuid.New()); !errors.Is(err, commerce.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Cancel(ctx, p, uuid.New(), 0, "valid-request-key-001", uuid.New()); !errors.Is(err, commerce.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.PlaceConfirmed(ctx, p, uuid.New(), -1, "EUR", "valid-request-key-001", uuid.New()); !errors.Is(err, commerce.ErrInvalid) {
		t.Fatal(err)
	}
}
