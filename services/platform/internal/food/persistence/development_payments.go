package persistence

import (
	"context"
	"database/sql"
	"errors"
	corestore "ocp.local/platform/internal/core/persistence"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/food/persistence/sqlgen"
	"ocp.local/platform/internal/payment"
)

func paymentOperation(ctx context.Context, tx *sql.Tx, order FoodOrder) (payment.Operation, error) {
	row, err := sqlgen.New(tx).GetDevelopmentPayment(ctx, sqlgen.GetDevelopmentPaymentParams{MerchantID: order.MerchantID, StoreID: order.StoreID, OrderID: order.ID})
	if errors.Is(err, sql.ErrNoRows) {
		return payment.Operation{State: "not_started"}, nil
	}
	return payment.Operation{State: row.State, Attempts: int(row.Attempts)}, err
}

func savePayment(ctx context.Context, tx *sql.Tx, order FoodOrder, operation payment.Operation) error {
	terms := payment.Terms{Subject: payment.Subject{Domain: capability.Food, MerchantID: order.MerchantID, StoreID: order.StoreID, ID: order.ID}, AmountMinor: order.Terms.Total, Currency: order.Terms.Currency}
	if err := terms.Validate(); err != nil {
		return err
	}
	// Attempts are bounded by the payment state machine before persistence.
	return sqlgen.New(tx).SaveDevelopmentPayment(ctx, sqlgen.SaveDevelopmentPaymentParams{MerchantID: terms.Subject.MerchantID, StoreID: terms.Subject.StoreID, OrderID: terms.Subject.ID, State: operation.State, Attempts: int32(operation.Attempts), AmountMinor: terms.AmountMinor, Currency: terms.Currency})
}

// The same order lock serializes payment simulation, cancellation, and merchant
// decisions. No network side effects occur inside this development transaction.
func ChangeDevelopmentPayment(ctx context.Context, tx *sql.Tx, p corestore.Principal, store, id uuid.UUID, expected int64, action string, requestID uuid.UUID) (FoodOrder, error) {
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
