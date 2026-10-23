package payment

import (
	"errors"

	"github.com/google/uuid"
	"ocp.local/platform/internal/core/capability"
)

// Subject identifies the domain-owned record requesting a financial operation.
// It does not grant access or imply that an order was accepted or fulfilled.
type Subject struct {
	Domain     capability.Domain
	MerchantID uuid.UUID
	StoreID    uuid.UUID
	ID         uuid.UUID
}

// Terms contains the immutable financial projection supplied by a domain.
// The domain must authorize and validate its subject before persisting payment.
type Terms struct {
	Subject     Subject
	AmountMinor int64
	Currency    string
}

var ErrTerms = errors.New("invalid payment terms")

func (t Terms) Validate() error {
	if !t.Subject.Domain.Valid() || t.Subject.MerchantID == uuid.Nil || t.Subject.StoreID == uuid.Nil || t.Subject.ID == uuid.Nil {
		return ErrTerms
	}
	// EUR is the currently supported financial contract, not an assumption that
	// all future domains or currencies have two decimal places.
	if t.Currency != "EUR" || t.AmountMinor < 0 || t.AmountMinor > 1_000_000_000_000 {
		return ErrTerms
	}
	return nil
}
