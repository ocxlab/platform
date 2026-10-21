package persistence

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// The caller holds the order's operational lock. Progress and its attributable
// event commit in the same transaction as the idempotent response.
func changeOrderEvent(ctx context.Context, tx *sql.Tx, p Principal, order *FoodOrder, event string, requestID uuid.UUID) error {
	order.Revision++
	_, err := tx.ExecContext(ctx, `UPDATE commerce.food_order_operations SET revision=$4,acceptance=$5,resolution=$6,fulfilment_progress=$7 WHERE merchant_id=$1 AND store_id=$2 AND order_id=$3`, order.MerchantID, order.StoreID, order.ID, order.Revision, order.Acceptance, order.Resolution, order.FulfilmentProgress)
	if err != nil {
		return err
	}
	var at time.Time
	err = tx.QueryRowContext(ctx, `INSERT INTO commerce.food_order_events(merchant_id,store_id,order_id,sequence,event,actor_id,request_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING created_at`, order.MerchantID, order.StoreID, order.ID, order.Revision, event, p.ID, requestID).Scan(&at)
	if err == nil {
		order.History = append(order.History, FoodOrderEvent{order.Revision, event, at})
	}
	return err
}
