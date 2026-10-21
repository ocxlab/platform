package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/offering/food"
)

var ErrQuoteExpired = errors.New("quote expired")
var ErrQuoteUsed = errors.New("quote already placed")

// Food owns these state names. They are not a universal commerce lifecycle.
type FoodOrder struct {
	ID                 uuid.UUID        `json:"id"`
	MerchantID         uuid.UUID        `json:"merchant_id"`
	StoreID            uuid.UUID        `json:"store_id"`
	Contract           string           `json:"contract"`
	Terms              FoodQuote        `json:"terms"`
	Fulfilment         string           `json:"fulfilment"`
	Acceptance         string           `json:"acceptance"`
	Payment            string           `json:"payment"`
	FulfilmentProgress string           `json:"fulfilment_progress"`
	Resolution         string           `json:"resolution"`
	CreatedAt          time.Time        `json:"created_at"`
	Revision           int64            `json:"revision,omitempty"`
	History            []FoodOrderEvent `json:"history,omitempty"`
}

// PlaceFoodOrder runs within the replay transaction. There are no external
// effects here: merchant decisions use the operational row; payment and kitchen
// work are not triggered by placement.
func PlaceFoodOrder(ctx context.Context, tx *sql.Tx, guest Principal, quoteID, requestID uuid.UUID) (FoodOrder, error) {
	var order FoodOrder
	// The store lock also serializes quote pruning, publication, and placement.
	current, err := publicOffering(ctx, tx, guest.MerchantID, guest.StoreID, true)
	if err != nil {
		return order, err
	}
	var body []byte
	var expires time.Time
	err = tx.QueryRowContext(ctx, `SELECT body,expires_at FROM commerce.food_quotes WHERE merchant_id=$1 AND store_id=$2 AND id=$3 AND guest_id=$4`, guest.MerchantID, guest.StoreID, quoteID, guest.ID).Scan(&body, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return order, ErrNotFound
	}
	if err != nil {
		return order, err
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT FROM commerce.food_orders WHERE merchant_id=$1 AND quote_id=$2)`, guest.MerchantID, quoteID).Scan(&exists); err != nil {
		return order, err
	}
	if exists {
		return order, ErrQuoteUsed
	}
	var now time.Time
	if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return order, err
	}
	if !expires.After(now) {
		return order, ErrQuoteExpired
	}
	var terms FoodQuote
	if err = json.Unmarshal(body, &terms); err != nil {
		return order, err
	}
	if current.Model != food.Model || terms.Model != food.Model {
		return order, food.ErrSelection
	}
	var lines []food.QuotedLine
	if err = json.Unmarshal(terms.Lines, &lines); err != nil {
		return order, err
	}
	selections := make([]food.SelectionLine, len(lines))
	for i, line := range lines {
		selections[i] = line.Selection
	}
	// Current availability/choice validity still applies. Retain issued prices and
	// names rather than repricing a live quote after a price-only publication edit.
	if _, _, err = food.Price(current.Document, selections); err != nil {
		return order, err
	}
	order = FoodOrder{ID: uuid.New(), MerchantID: guest.MerchantID, StoreID: guest.StoreID, Contract: "ocp.food.order.v1", Terms: terms, Fulfilment: "pickup", Acceptance: "pending", Payment: "not_started", FulfilmentProgress: "not_started", Resolution: "open", CreatedAt: now}
	body, err = json.Marshal(order)
	if err != nil {
		return order, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO commerce.food_orders(merchant_id,store_id,id,guest_id,quote_id,body,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, guest.MerchantID, guest.StoreID, order.ID, guest.ID, quoteID, string(body), now)
	if err != nil {
		return order, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO commerce.food_order_operations(merchant_id,store_id,order_id) VALUES($1,$2,$3)`, guest.MerchantID, guest.StoreID, order.ID)
	if err != nil {
		return order, err
	}
	order.Revision = 1
	_, err = tx.ExecContext(ctx, `INSERT INTO commerce.food_order_events(merchant_id,store_id,order_id,sequence,event,actor_id,request_id) VALUES($1,$2,$3,1,'placed',$4,$5)`, guest.MerchantID, guest.StoreID, order.ID, guest.ID, requestID)
	return order, err
}

func GetGuestFoodOrder(ctx context.Context, db *sql.DB, guest Principal, id uuid.UUID) (FoodOrder, error) {
	tx, err := TenantTransaction(ctx, db, guest.MerchantID)
	if err != nil {
		return FoodOrder{}, err
	}
	defer tx.Rollback()
	if err = checkGuest(ctx, tx, guest); err != nil {
		return FoodOrder{}, err
	}
	order, err := readFoodOrder(ctx, tx, guest, guest.StoreID, id, false)
	if err != nil {
		return order, err
	}
	return order, tx.Commit()
}

func GetGuestFoodQuote(ctx context.Context, db *sql.DB, guest Principal, id uuid.UUID) (FoodQuote, error) {
	tx, err := TenantTransaction(ctx, db, guest.MerchantID)
	if err != nil {
		return FoodQuote{}, err
	}
	defer tx.Rollback()
	if err = checkGuest(ctx, tx, guest); err != nil {
		return FoodQuote{}, err
	}
	var body []byte
	err = tx.QueryRowContext(ctx, `SELECT body FROM commerce.food_quotes WHERE merchant_id=$1 AND store_id=$2 AND id=$3 AND guest_id=$4`, guest.MerchantID, guest.StoreID, id, guest.ID).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return FoodQuote{}, ErrNotFound
	}
	if err != nil {
		return FoodQuote{}, err
	}
	var quote FoodQuote
	if err = json.Unmarshal(body, &quote); err != nil {
		return quote, err
	}
	return quote, tx.Commit()
}
