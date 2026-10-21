package persistence

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// Merchant reports of physical work are separate from financial outcomes. This
// pickup slice deliberately requires payment before work starts; cash-on-pickup
// and merchant-configured payment conditions are separate future capabilities.
func ProgressFoodFulfilment(ctx context.Context, tx *sql.Tx, p Principal, store, id uuid.UUID, expected int64, action string, requestID uuid.UUID) (FoodOrder, error) {
	order, err := readFoodOrder(ctx, tx, p, store, id, true)
	if err != nil {
		return order, err
	}
	if order.Revision != expected {
		return order, &OrderRevisionConflict{Current: order.Revision}
	}
	if p.Guest || order.Resolution != "open" || order.Acceptance != "accepted" || order.Payment != "paid" || order.Fulfilment != "pickup" {
		return order, ErrOrderTransition
	}
	event := ""
	switch {
	case action == "start_preparation" && order.FulfilmentProgress == "not_started":
		order.FulfilmentProgress, event = "preparing", "preparation_started"
	case action == "mark_ready" && order.FulfilmentProgress == "preparing":
		order.FulfilmentProgress, event = "ready", "ready_for_pickup"
	case action == "record_collection" && order.FulfilmentProgress == "ready":
		order.FulfilmentProgress, event = "collected", "collected"
	default:
		return order, ErrOrderTransition
	}
	err = changeOrderEvent(ctx, tx, p, &order, event, requestID)
	return order, err
}
