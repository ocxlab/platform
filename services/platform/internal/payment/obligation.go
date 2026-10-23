package payment

import (
	"context"
	"database/sql"
	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/payment/persistence/sqlgen"
)

// RecordPayLater persists a financial obligation in the caller's decision
// transaction. It never reports a charge, authorization, or payment success.
func RecordPayLater(ctx context.Context, tx *sql.Tx, t Terms) error {
	if t.Validate() != nil || (t.Subject.Domain != capability.Shopping && t.Subject.Domain != capability.Lodging) {
		return ErrTerms
	}
	s := t.Subject
	return sqlgen.New(tx).InsertPayLaterObligation(ctx, sqlgen.InsertPayLaterObligationParams{MerchantID: s.MerchantID, StoreID: s.StoreID, Domain: string(s.Domain), SubjectID: s.ID, AmountMinor: t.AmountMinor, Currency: t.Currency})
}
