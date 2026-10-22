package payment

import (
	"context"
	"github.com/google/uuid"
	"ocp.local/platform/internal/capability"
	"testing"
)

func TestPayLaterRejectsInvalidTermsBeforeStorage(t *testing.T) {
	base := Terms{Subject: Subject{MerchantID: uuid.New(), StoreID: uuid.New(), ID: uuid.New(), Domain: capability.Shopping}, AmountMinor: 100, Currency: "EUR"}
	for _, change := range []func(*Terms){func(t *Terms) { t.Subject.Domain = capability.Food }, func(t *Terms) { t.Subject.Domain = "unknown" }, func(t *Terms) { t.Subject.Domain = capability.Lodging; t.Currency = "USD" }, func(t *Terms) { t.Subject.ID = uuid.Nil }} {
		terms := base
		change(&terms)
		if RecordPayLater(context.Background(), nil, terms) != ErrTerms {
			t.Fatal("invalid financial obligation reached storage")
		}
	}
}
