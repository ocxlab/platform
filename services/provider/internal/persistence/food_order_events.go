package persistence

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"ocp.local/platform/internal/persistence/sqlgen"
)

// The caller holds the order's operational lock. Progress and its attributable
// event commit in the same transaction as the idempotent response.
func changeOrderEvent(ctx context.Context, tx *sql.Tx, p Principal, order *FoodOrder, event string, requestID uuid.UUID) error {
	order.Revision++
	queries := sqlgen.New(tx)
	err := queries.UpdateFoodOrderOperations(ctx, sqlgen.UpdateFoodOrderOperationsParams{MerchantID: order.MerchantID, StoreID: order.StoreID, OrderID: order.ID, Revision: order.Revision, Acceptance: order.Acceptance, Resolution: order.Resolution, FulfilmentProgress: order.FulfilmentProgress})
	if err != nil {
		return err
	}
	at, err := queries.InsertFoodOrderEvent(ctx, sqlgen.InsertFoodOrderEventParams{MerchantID: order.MerchantID, StoreID: order.StoreID, OrderID: order.ID, Sequence: order.Revision, Event: event, ActorID: p.ID, RequestID: requestID})
	if err == nil {
		order.History = append(order.History, FoodOrderEvent{order.Revision, event, at})
	}
	return err
}
