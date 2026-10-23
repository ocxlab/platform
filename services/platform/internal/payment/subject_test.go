package payment

import (
	"github.com/google/uuid"
	"ocp.local/platform/internal/core/capability"
	"testing"
)

func TestPaymentTerms(t *testing.T) {
	base := Terms{Subject: Subject{Domain: capability.Food, MerchantID: uuid.New(), StoreID: uuid.New(), ID: uuid.New()}, AmountMinor: 100, Currency: "EUR"}
	for _, d := range []capability.Domain{capability.Food, capability.Shopping, capability.Lodging} {
		for _, amount := range []int64{0, 1, 1_000_000_000_000} {
			terms := base
			terms.Subject.Domain = d
			terms.AmountMinor = amount
			if err := terms.Validate(); err != nil {
				t.Fatalf("%s/%d rejected: %v", d, amount, err)
			}
		}
	}
	for _, change := range []func(*Terms){
		func(t *Terms) { t.Subject.Domain = "unknown" }, func(t *Terms) { t.Subject.MerchantID = uuid.Nil },
		func(t *Terms) { t.Subject.StoreID = uuid.Nil }, func(t *Terms) { t.Subject.ID = uuid.Nil },
		func(t *Terms) { t.Currency = "USD" }, func(t *Terms) { t.Currency = "" },
		func(t *Terms) { t.AmountMinor = -1 }, func(t *Terms) { t.AmountMinor = 1_000_000_000_001 },
	} {
		terms := base
		change(&terms)
		if terms.Validate() != ErrTerms {
			t.Fatalf("invalid terms accepted: %+v", terms)
		}
	}
}
