package foodapp

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	commerce "ocp.local/platform/internal/core/application"
	"ocp.local/platform/internal/core/persistence"
	foodstore "ocp.local/platform/internal/food/persistence"
)

// Orders owns merchant intentions shared by REST and MCP. Persistence retains
// the locked Food state machine, durable work, attributable events, and replay.
type Orders struct{ DB *sql.DB }

func (s Orders) List(ctx context.Context, p persistence.Principal, store uuid.UUID, after *string, limit int) (foodstore.FoodOrderPage, error) {
	if !p.AuthorizesMerchant(p.MerchantID, "order:read") {
		return foodstore.FoodOrderPage{}, commerce.ErrForbidden
	}
	if store == uuid.Nil || limit < 1 || limit > 100 {
		return foodstore.FoodOrderPage{}, commerce.ErrInvalid
	}
	return foodstore.ListFoodOrders(ctx, s.DB, p, store, after, limit)
}
func (s Orders) Get(ctx context.Context, p persistence.Principal, store, id uuid.UUID) (foodstore.FoodOrder, error) {
	if !p.AuthorizesMerchant(p.MerchantID, "order:read") {
		return foodstore.FoodOrder{}, commerce.ErrForbidden
	}
	if store == uuid.Nil || id == uuid.Nil {
		return foodstore.FoodOrder{}, commerce.ErrInvalid
	}
	return foodstore.GetFoodOrder(ctx, s.DB, p, store, id)
}
func (s Orders) Change(ctx context.Context, p persistence.Principal, store, id uuid.UUID, expected int64, action, key string, requestID uuid.UUID) (persistence.Result, error) {
	if !p.AuthorizesMerchant(p.MerchantID, "order:manage") {
		return persistence.Result{}, commerce.ErrForbidden
	}
	if store == uuid.Nil || id == uuid.Nil || expected < 1 || requestID == uuid.Nil || !commerce.ValidRequestKey(key) {
		return persistence.Result{}, commerce.ErrInvalid
	}
	operation, field := "", ""
	switch action {
	case "accept", "reject":
		operation, field = "decideFoodOrder", "decision"
	case "start_preparation", "mark_ready", "record_collection":
		operation, field = "progressFoodFulfilment", "action"
	default:
		return persistence.Result{}, commerce.ErrInvalid
	}
	// Match existing REST canonical field order and replay operation domains.
	canonical, _ := json.Marshal(map[string]any{field: action, "expected_revision": expected})
	return persistence.Mutate(ctx, s.DB, p, operation, store.String()+"/"+id.String(), key, canonical, func(tx *sql.Tx) (any, error) {
		if field == "decision" {
			return foodstore.ChangeFoodOrder(ctx, tx, p, store, id, expected, action, requestID)
		}
		return foodstore.ProgressFoodFulfilment(ctx, tx, p, store, id, expected, action, requestID)
	})
}
