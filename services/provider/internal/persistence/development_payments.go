package persistence

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"ocp.local/platform/internal/payment"
)

func paymentOperation(ctx context.Context, tx *sql.Tx, order FoodOrder) (payment.Operation, error) {
	operation := payment.Operation{State: "not_started"}
	err := tx.QueryRowContext(ctx, `SELECT state,attempts FROM payment.development_operations WHERE merchant_id=$1 AND store_id=$2 AND order_id=$3`, order.MerchantID, order.StoreID, order.ID).Scan(&operation.State, &operation.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return payment.Operation{State: "not_started"}, nil
	}
	return operation, err
}

func savePayment(ctx context.Context, tx *sql.Tx, order FoodOrder, operation payment.Operation) error {
	terms := payment.Terms{Subject: payment.Subject{Domain: capability.Food, MerchantID: order.MerchantID, StoreID: order.StoreID, ID: order.ID}, AmountMinor: order.Terms.Total, Currency: order.Terms.Currency}
	if err := terms.Validate(); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO payment.development_operations(merchant_id,store_id,order_id,state,attempts,amount_minor,currency)
 VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(merchant_id,store_id,order_id) DO UPDATE SET state=EXCLUDED.state,attempts=EXCLUDED.attempts`, terms.Subject.MerchantID, terms.Subject.StoreID, terms.Subject.ID, operation.State, operation.Attempts, terms.AmountMinor, terms.Currency)
	return err
}

// The same order lock serializes payment simulation, cancellation, and merchant
// decisions. No network side effects occur inside this development transaction.
func ChangeDevelopmentPayment(ctx context.Context, tx *sql.Tx, p Principal, store, id uuid.UUID, expected int64, action string, requestID uuid.UUID) (FoodOrder, error) {
	order, err := readFoodOrder(ctx, tx, p, store, id, true)
	if err != nil {
		return order, err
	}
	if order.Revision != expected {
		return order, &OrderRevisionConflict{Current: order.Revision}
	}
	if !p.Guest || order.Resolution != "open" || order.Acceptance != "accepted" || order.FulfilmentProgress != "not_started" || (order.PaymentProvider != "" && order.PaymentProvider != "development") {
		return order, ErrOrderTransition
	}
	operation, err := paymentOperation(ctx, tx, order)
	if err != nil {
		return order, err
	}
	next, err := payment.DevelopmentTransition(operation, action)
	if err != nil {
		return order, ErrOrderTransition
	}
	if err = savePayment(ctx, tx, order, next); err != nil {
		return order, err
	}
	order.Payment = next.State
	order.PaymentProvider = "development"
	event := map[string]string{"start": "payment_started", "succeed": "payment_paid", "decline": "payment_failed", "abandon": "payment_abandoned"}[action]
	err = changeOrderEvent(ctx, tx, p, &order, event, requestID)
	return order, err
}
