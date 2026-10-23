package foodapp

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	commerce "ocp.local/platform/internal/core/application"
	core "ocp.local/platform/internal/core/persistence"
	food "ocp.local/platform/internal/food/offering"
	store "ocp.local/platform/internal/food/persistence"
	"strings"
	"unicode/utf8"
)

// Checkout owns customer operations shared by REST and intent-based adapters.
// Quotes are immutable, owned basket snapshots, not reservations or payments.
type Checkout struct{ DB *sql.DB }

func (s Checkout) Quote(ctx context.Context, p core.Principal, merchant, shop uuid.UUID, generation int64, lines []food.SelectionLine, locale, note string, guest bool) (store.FoodQuote, error) {
	if generation < 1 || !validNote(note) || (locale != "" && locale != "de" && locale != "en" && locale != "ar") {
		return store.FoodQuote{}, commerce.ErrInvalid
	}
	if guest && (!p.Guest || !p.Allows("quote:create") || p.MerchantID != merchant || p.StoreID != shop) {
		return store.FoodQuote{}, commerce.ErrForbidden
	}
	for _, line := range lines {
		if !validNote(line.Note) {
			return store.FoodQuote{}, commerce.ErrInvalid
		}
	}
	calculate := func(p core.PublicOffering) (store.QuoteCalculation, error) {
		priced, total, expires, err := food.PriceModel(p.Model, p.Document, lines, locale, p.EvaluatedAt)
		if err != nil {
			return store.QuoteCalculation{}, err
		}
		encoded, err := json.Marshal(priced)
		return store.QuoteCalculation{Lines: encoded, Total: total, Note: note, ExpiresAt: expires}, err
	}
	if guest {
		return store.CreateGuestFoodQuote(ctx, s.DB, p, generation, calculate)
	}
	return store.CreateFoodQuote(ctx, s.DB, merchant, shop, generation, calculate)
}

func validNote(note string) bool {
	return utf8.ValidString(note) && utf8.RuneCountInString(note) <= 500 && !strings.ContainsRune(note, 0)
}

func (s Checkout) Place(ctx context.Context, p core.Principal, quote uuid.UUID, key string, request uuid.UUID) (core.Result, error) {
	return s.place(ctx, p, quote, key, request, nil)
}

func (s Checkout) PlaceConfirmed(ctx context.Context, p core.Principal, quote uuid.UUID, total int64, currency, key string, request uuid.UUID) (core.Result, error) {
	if total < 0 || currency != "EUR" {
		return core.Result{}, commerce.ErrInvalid
	}
	check := func(order store.FoodOrder) error {
		if order.Terms.Total != total || order.Terms.Currency != currency {
			return commerce.ErrInvalid
		}
		return nil
	}
	result, err := s.place(ctx, p, quote, key, request, check)
	// Replays skip mutation callbacks. Verify their retained terms too, without
	// depending on an expired/pruned quote or today's publication.
	if err == nil {
		var order store.FoodOrder
		if json.Unmarshal(result.Body, &order) != nil {
			return core.Result{}, commerce.ErrInvalid
		}
		if err = check(order); err != nil {
			return core.Result{}, err
		}
	}
	return result, err
}

func (s Checkout) place(ctx context.Context, p core.Principal, quote uuid.UUID, key string, request uuid.UUID, check func(store.FoodOrder) error) (core.Result, error) {
	if !p.Guest || !p.Allows("order:create") {
		return core.Result{}, commerce.ErrForbidden
	}
	if quote == uuid.Nil || request == uuid.Nil || !commerce.ValidRequestKey(key) {
		return core.Result{}, commerce.ErrInvalid
	}
	body, _ := json.Marshal(struct {
		Fulfilment string    `json:"fulfilment"`
		QuoteID    uuid.UUID `json:"quote_id"`
	}{"pickup", quote})
	return core.Mutate(ctx, s.DB, p, "placeFoodOrder", p.StoreID.String(), key, body, func(tx *sql.Tx) (any, error) {
		order, err := store.PlaceFoodOrder(ctx, tx, p, quote, request)
		if err == nil && check != nil {
			err = check(order)
		}
		return order, err
	})
}

func (s Checkout) Cancel(ctx context.Context, p core.Principal, id uuid.UUID, revision int64, key string, request uuid.UUID) (core.Result, error) {
	if !p.Guest || !p.Allows("order:cancel") {
		return core.Result{}, commerce.ErrForbidden
	}
	if id == uuid.Nil || revision < 1 || request == uuid.Nil || !commerce.ValidRequestKey(key) {
		return core.Result{}, commerce.ErrInvalid
	}
	body, _ := json.Marshal(map[string]any{"expected_revision": revision})
	return core.Mutate(ctx, s.DB, p, "cancelGuestFoodOrder", p.StoreID.String()+"/"+id.String(), key, body, func(tx *sql.Tx) (any, error) {
		return store.ChangeFoodOrder(ctx, tx, p, p.StoreID, id, revision, "cancel", request)
	})
}
