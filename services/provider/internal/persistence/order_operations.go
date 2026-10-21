package persistence

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type OrderRevisionConflict struct{ Current int64 }

func (e *OrderRevisionConflict) Error() string { return "order revision changed" }

var ErrOrderTransition = errors.New("order action unavailable")
var ErrOrderCursor = errors.New("invalid order cursor")

type FoodOrderEvent struct {
	Sequence  int64     `json:"sequence"`
	Event     string    `json:"event"`
	CreatedAt time.Time `json:"created_at"`
}

type FoodOrderPage struct {
	Items     []FoodOrder `json:"items"`
	NextAfter *string     `json:"next_after,omitempty"`
}

func orderCursor(order FoodOrder) string {
	return base64.RawURLEncoding.EncodeToString([]byte(order.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + order.ID.String()))
}
func parseOrderCursor(cursor string) (time.Time, uuid.UUID, error) {
	if len(cursor) > 128 {
		return time.Time{}, uuid.Nil, ErrOrderCursor
	}
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrOrderCursor
	}
	parts := strings.Split(string(data), "|")
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, ErrOrderCursor
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, ErrOrderCursor
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, ErrOrderCursor
	}
	return at, id, nil
}

// Lock the mutable progress row, never the immutable placement body. Every
// future payment/kitchen transition must use this same per-order serialization.
func readFoodOrder(ctx context.Context, tx *sql.Tx, p Principal, store, id uuid.UUID, lock bool) (FoodOrder, error) {
	var order FoodOrder
	var body []byte
	query := `SELECT o.body,s.revision,s.acceptance,s.resolution FROM commerce.food_orders o
 JOIN commerce.food_order_operations s ON (s.merchant_id,s.store_id,s.order_id)=(o.merchant_id,o.store_id,o.id)
 WHERE o.merchant_id=$1 AND o.store_id=$2 AND o.id=$3 AND ($4::uuid IS NULL OR o.guest_id=$4)`
	if lock {
		query += ` FOR UPDATE OF s`
	} else {
		query += ` FOR SHARE OF s`
	}
	var guest *uuid.UUID
	if p.Guest {
		guest = &p.ID
	}
	var revision int64
	var acceptance, resolution string
	err := tx.QueryRowContext(ctx, query, p.MerchantID, store, id, guest).Scan(&body, &revision, &acceptance, &resolution)
	if errors.Is(err, sql.ErrNoRows) {
		return order, ErrNotFound
	}
	if err != nil {
		return order, err
	}
	if err = json.Unmarshal(body, &order); err != nil {
		return order, err
	}
	order.Revision, order.Acceptance, order.Resolution = revision, acceptance, resolution
	rows, err := tx.QueryContext(ctx, `SELECT sequence,event,created_at FROM commerce.food_order_events WHERE merchant_id=$1 AND store_id=$2 AND order_id=$3 ORDER BY sequence`, p.MerchantID, store, id)
	if err != nil {
		return order, err
	}
	defer rows.Close()
	order.History = make([]FoodOrderEvent, 0)
	for rows.Next() {
		var event FoodOrderEvent
		if err = rows.Scan(&event.Sequence, &event.Event, &event.CreatedAt); err != nil {
			return order, err
		}
		order.History = append(order.History, event)
	}
	return order, rows.Err()
}

func GetFoodOrder(ctx context.Context, db *sql.DB, p Principal, store, id uuid.UUID) (FoodOrder, error) {
	tx, err := TenantTransaction(ctx, db, p.MerchantID)
	if err != nil {
		return FoodOrder{}, err
	}
	defer tx.Rollback()
	order, err := readFoodOrder(ctx, tx, p, store, id, false)
	if err != nil {
		return order, err
	}
	return order, tx.Commit()
}

// Decision, event, and replay all share Mutate's transaction. Neither endpoint
// claims payment or sends a kitchen instruction.
func ChangeFoodOrder(ctx context.Context, tx *sql.Tx, p Principal, store, id uuid.UUID, expected int64, action string, requestID uuid.UUID) (FoodOrder, error) {
	order, err := readFoodOrder(ctx, tx, p, store, id, true)
	if err != nil {
		return order, err
	}
	if order.Revision != expected {
		return order, &OrderRevisionConflict{Current: order.Revision}
	}
	if order.Resolution != "open" || order.Payment != "not_started" || order.FulfilmentProgress != "not_started" {
		return order, ErrOrderTransition
	}
	event := ""
	switch action {
	case "accept":
		if p.Guest || order.Acceptance != "pending" {
			return order, ErrOrderTransition
		}
		order.Acceptance = "accepted"
		event = "accepted"
	case "reject":
		if p.Guest || order.Acceptance != "pending" {
			return order, ErrOrderTransition
		}
		order.Acceptance = "rejected"
		order.Resolution = "rejected"
		event = "rejected"
	case "cancel":
		if !p.Guest {
			return order, ErrOrderTransition
		}
		order.Resolution = "cancelled"
		event = "cancelled"
	default:
		return order, ErrOrderTransition
	}
	order.Revision++
	_, err = tx.ExecContext(ctx, `UPDATE commerce.food_order_operations SET revision=$4,acceptance=$5,resolution=$6 WHERE merchant_id=$1 AND store_id=$2 AND order_id=$3`, p.MerchantID, store, id, order.Revision, order.Acceptance, order.Resolution)
	if err != nil {
		return order, err
	}
	var at time.Time
	err = tx.QueryRowContext(ctx, `INSERT INTO commerce.food_order_events(merchant_id,store_id,order_id,sequence,event,actor_id,request_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING created_at`, p.MerchantID, store, id, order.Revision, event, p.ID, requestID).Scan(&at)
	if err != nil {
		return order, err
	}
	order.History = append(order.History, FoodOrderEvent{order.Revision, event, at})
	return order, nil
}

func ListFoodOrders(ctx context.Context, db *sql.DB, p Principal, store uuid.UUID, after *string, limit int) (FoodOrderPage, error) {
	page := FoodOrderPage{Items: make([]FoodOrder, 0)}
	if limit < 1 || limit > 100 {
		return page, errors.New("order page limit outside bounds")
	}
	var at *time.Time
	var id *uuid.UUID
	if after != nil {
		value, key, err := parseOrderCursor(*after)
		if err != nil {
			return page, err
		}
		at, id = &value, &key
	}
	tx, err := TenantTransaction(ctx, db, p.MerchantID)
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT FROM commerce.stores WHERE merchant_id=$1 AND id=$2)`, p.MerchantID, store).Scan(&exists); err != nil {
		return page, err
	}
	if !exists {
		return page, ErrNotFound
	}
	rows, err := tx.QueryContext(ctx, `SELECT o.body,s.revision,s.acceptance,s.resolution FROM commerce.food_orders o
 JOIN commerce.food_order_operations s ON (s.merchant_id,s.store_id,s.order_id)=(o.merchant_id,o.store_id,o.id)
 WHERE o.merchant_id=$1 AND o.store_id=$2 AND ($3::timestamptz IS NULL OR (o.created_at,o.id)<($3,$4::uuid)) ORDER BY o.created_at DESC,o.id DESC LIMIT $5`, p.MerchantID, store, at, id, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var order FoodOrder
		var body []byte
		var revision int64
		var acceptance, resolution string
		if err = rows.Scan(&body, &revision, &acceptance, &resolution); err != nil {
			return page, err
		}
		if err = json.Unmarshal(body, &order); err != nil {
			return page, err
		}
		order.Revision, order.Acceptance, order.Resolution = revision, acceptance, resolution
		page.Items = append(page.Items, order)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		cursor := orderCursor(page.Items[limit-1])
		page.NextAfter = &cursor
	}
	return page, tx.Commit()
}
