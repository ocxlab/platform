package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	corestore "ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/database"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"ocp.local/platform/internal/food/persistence/sqlgen"
	"ocp.local/platform/internal/offering/food"
)

var ErrQuoteExpired = errors.New("quote expired")
var ErrQuoteUsed = errors.New("quote already placed")

// Food owns these state names. They are not a universal commerce lifecycle.
type FoodOrder struct {
	Reference          string           `json:"reference,omitempty"`
	ID                 uuid.UUID        `json:"id"`
	MerchantID         uuid.UUID        `json:"merchant_id"`
	StoreID            uuid.UUID        `json:"store_id"`
	Contract           string           `json:"contract"`
	Terms              FoodQuote        `json:"terms"`
	Fulfilment         string           `json:"fulfilment"`
	Acceptance         string           `json:"acceptance"`
	Payment            string           `json:"payment"`
	PaymentProvider    string           `json:"payment_provider,omitempty"`
	FulfilmentProgress string           `json:"fulfilment_progress"`
	Resolution         string           `json:"resolution"`
	CreatedAt          time.Time        `json:"created_at"`
	Revision           int64            `json:"revision,omitempty"`
	History            []FoodOrderEvent `json:"history,omitempty"`
}

// PlaceFoodOrder runs within the replay transaction. There are no external
// effects here: merchant decisions use the operational row; payment and kitchen
// work are not triggered by placement.
func PlaceFoodOrder(ctx context.Context, tx *sql.Tx, guest corestore.Principal, quoteID, requestID uuid.UUID) (FoodOrder, error) {
	var order FoodOrder
	// The store lock also serializes quote pruning, publication, and placement.
	current, err := corestore.PublicOfferingTx(ctx, tx, guest.MerchantID, guest.StoreID, true)
	if err != nil {
		return order, err
	}
	ordering, err := foodOrdering(ctx, tx, guest.MerchantID, guest.StoreID, false)
	if err != nil {
		return order, err
	}
	if !ordering.AcceptingOrders {
		return order, ErrOrderingPaused
	}
	queries := sqlgen.New(tx)
	quote, err := queries.GetGuestFoodQuote(ctx, sqlgen.GetGuestFoodQuoteParams{MerchantID: guest.MerchantID, StoreID: guest.StoreID, ID: quoteID, GuestID: uuid.NullUUID{UUID: guest.ID, Valid: true}})
	if errors.Is(err, sql.ErrNoRows) {
		return order, corestore.ErrNotFound
	}
	if err != nil {
		return order, err
	}
	exists, err := queries.FoodQuoteUsed(ctx, sqlgen.FoodQuoteUsedParams{MerchantID: guest.MerchantID, QuoteID: quoteID})
	if err != nil {
		return order, err
	}
	if exists {
		return order, ErrQuoteUsed
	}
	now, err := queries.DatabaseTime(ctx)
	if err != nil {
		return order, err
	}
	if !quote.ExpiresAt.After(now) {
		return order, ErrQuoteExpired
	}
	var terms FoodQuote
	if err = json.Unmarshal(quote.Body, &terms); err != nil {
		return order, err
	}
	if current.Model != terms.Model || current.Model != food.Model {
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
	if _, _, _, err = food.PriceModel(current.Model, current.Document, selections, "", now); err != nil {
		return order, err
	}
	order = FoodOrder{ID: uuid.New(), MerchantID: guest.MerchantID, StoreID: guest.StoreID, Contract: "ocp.food.order.v1", Terms: terms, Fulfilment: "pickup", Acceptance: "pending", Payment: "not_started", FulfilmentProgress: "not_started", Resolution: "open", CreatedAt: now}
	body, err := json.Marshal(order)
	if err != nil {
		return order, err
	}
	number, err := queries.InsertFoodOrder(ctx, sqlgen.InsertFoodOrderParams{MerchantID: guest.MerchantID, StoreID: guest.StoreID, ID: order.ID, GuestID: guest.ID, QuoteID: quoteID, Body: body, CreatedAt: now})
	if err != nil {
		return order, err
	}
	if !number.Valid {
		return order, errors.New("food order has no display number")
	}
	err = queries.InsertFoodOrderOperations(ctx, sqlgen.InsertFoodOrderOperationsParams{MerchantID: guest.MerchantID, StoreID: guest.StoreID, OrderID: order.ID})
	if err != nil {
		return order, err
	}
	order.Revision = 1
	order.Reference = orderReference(number.Int64)
	_, err = queries.InsertFoodOrderEvent(ctx, sqlgen.InsertFoodOrderEventParams{MerchantID: guest.MerchantID, StoreID: guest.StoreID, OrderID: order.ID, Sequence: 1, Event: "placed", ActorID: guest.ID, RequestID: requestID})
	if err != nil {
		return order, err
	}
	if ordering.AcceptanceMode == "automatic" {
		// Attribute acceptance to the merchant grant that enabled the policy,
		// never to the customer placing an order. Payment/preparation stay separate.
		if !ordering.actor.Valid {
			return order, errors.New("automatic policy has no merchant actor")
		}
		err = queries.UpdateFoodOrderOperations(ctx, sqlgen.UpdateFoodOrderOperationsParams{MerchantID: guest.MerchantID, StoreID: guest.StoreID, OrderID: order.ID, Revision: 2, Acceptance: "accepted", Resolution: order.Resolution, FulfilmentProgress: order.FulfilmentProgress})
		if err != nil {
			return order, err
		}
		_, err = queries.InsertFoodOrderEvent(ctx, sqlgen.InsertFoodOrderEventParams{MerchantID: guest.MerchantID, StoreID: guest.StoreID, OrderID: order.ID, Sequence: 2, Event: "accepted", ActorID: ordering.actor.UUID, RequestID: requestID})
		order.Acceptance = "accepted"
		order.Revision = 2
	}
	return order, err
}

func GetGuestFoodOrder(ctx context.Context, db *sql.DB, guest corestore.Principal, id uuid.UUID) (FoodOrder, error) {
	return getGuestFoodOrder(ctx, db, guest, id, "")
}

func GetGuestFoodOrderByReference(ctx context.Context, db *sql.DB, guest corestore.Principal, reference string) (FoodOrder, error) {
	return getGuestFoodOrder(ctx, db, guest, uuid.Nil, reference)
}

func getGuestFoodOrder(ctx context.Context, db *sql.DB, guest corestore.Principal, id uuid.UUID, reference string) (FoodOrder, error) {
	tx, err := database.TenantTransaction(ctx, db, guest.MerchantID)
	if err != nil {
		return FoodOrder{}, err
	}
	defer tx.Rollback()
	if err = corestore.CheckGuest(ctx, tx, guest); err != nil {
		return FoodOrder{}, err
	}
	if reference != "" {
		number, parseErr := strconv.ParseInt(strings.TrimPrefix(reference, "O-"), 10, 64)
		if parseErr != nil || number < 1 || orderReference(number) != reference {
			return FoodOrder{}, corestore.ErrNotFound
		}
		id, err = sqlgen.New(tx).GetGuestFoodOrderID(ctx, sqlgen.GetGuestFoodOrderIDParams{MerchantID: guest.MerchantID, StoreID: guest.StoreID, GuestID: guest.ID, DisplayNumber: sql.NullInt64{Int64: number, Valid: true}})
		if errors.Is(err, sql.ErrNoRows) {
			return FoodOrder{}, corestore.ErrNotFound
		}
		if err != nil {
			return FoodOrder{}, err
		}
	}
	order, err := readFoodOrder(ctx, tx, guest, guest.StoreID, id, false)
	if err != nil {
		return order, err
	}
	return order, tx.Commit()
}

func GetGuestFoodQuote(ctx context.Context, db *sql.DB, guest corestore.Principal, id uuid.UUID) (FoodQuote, error) {
	tx, err := database.TenantTransaction(ctx, db, guest.MerchantID)
	if err != nil {
		return FoodQuote{}, err
	}
	defer tx.Rollback()
	if err = corestore.CheckGuest(ctx, tx, guest); err != nil {
		return FoodQuote{}, err
	}
	row, err := sqlgen.New(tx).GetGuestFoodQuote(ctx, sqlgen.GetGuestFoodQuoteParams{MerchantID: guest.MerchantID, StoreID: guest.StoreID, ID: id, GuestID: uuid.NullUUID{UUID: guest.ID, Valid: true}})
	if errors.Is(err, sql.ErrNoRows) {
		return FoodQuote{}, corestore.ErrNotFound
	}
	if err != nil {
		return FoodQuote{}, err
	}
	var quote FoodQuote
	if err = json.Unmarshal(row.Body, &quote); err != nil {
		return quote, err
	}
	return quote, tx.Commit()
}
