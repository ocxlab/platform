package payment

import (
	"context"
	"database/sql"
	"ocp.local/platform/internal/capability"
)

// RecordPayLater persists a financial obligation in the caller's decision
// transaction. It never reports a charge, authorization, or payment success.
func RecordPayLater(ctx context.Context, tx *sql.Tx, t Terms) error {
	if t.Validate() != nil || (t.Subject.Domain != capability.Shopping && t.Subject.Domain != capability.Lodging) {
		return ErrTerms
	}
	s := t.Subject
	_, err := tx.ExecContext(ctx, `INSERT INTO payments.obligations(merchant_id,store_id,domain,subject_id,amount_minor,currency,condition,state) VALUES($1,$2,$3,$4,$5,$6,'pay_later','outstanding')`, s.MerchantID, s.StoreID, s.Domain, s.ID, t.AmountMinor, t.Currency)
	return err
}
