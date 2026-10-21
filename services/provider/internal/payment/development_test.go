package payment

import (
	"errors"
	"testing"
)

func TestDevelopmentTransitions(t *testing.T) {
	valid := map[string]map[string]string{
		"not_started":       {"start": "awaiting_customer", "cancel_order": "not_started"},
		"failed":            {"start": "awaiting_customer", "cancel_order": "failed"},
		"awaiting_customer": {"succeed": "paid", "decline": "failed", "abandon": "not_started", "cancel_order": "not_started"},
		"paid":              {"cancel_order": "refunded"},
	}
	for _, state := range []string{"not_started", "failed", "awaiting_customer", "paid", "refunded", "unknown"} {
		for _, action := range []string{"start", "succeed", "decline", "abandon", "cancel_order", "unknown"} {
			t.Run(state+"/"+action, func(t *testing.T) {
				before := Operation{State: state, Attempts: 2}
				got, err := DevelopmentTransition(before, action)
				want, ok := valid[state][action]
				if !ok {
					if !errors.Is(err, ErrTransition) || got != before {
						t.Fatalf("invalid transition changed state: %+v %v", got, err)
					}
					return
				}
				attempts := 2
				if action == "start" {
					attempts++
				}
				if err != nil || got != (Operation{State: want, Attempts: attempts}) {
					t.Fatalf("got %+v %v", got, err)
				}
			})
		}
	}
}

func TestDevelopmentAttemptBoundary(t *testing.T) {
	if MaxAttempts != 10 {
		t.Fatal("documented attempt limit changed")
	}
	for _, state := range []string{"not_started", "failed"} {
		for _, attempts := range []int{0, 9, 10, 11} {
			before := Operation{State: state, Attempts: attempts}
			got, err := DevelopmentTransition(before, "start")
			if attempts < 10 {
				if err != nil || got.State != "awaiting_customer" || got.Attempts != attempts+1 {
					t.Fatalf("boundary rejected: %+v %v", got, err)
				}
			} else if !errors.Is(err, ErrTransition) || got != before {
				t.Fatalf("limit not enforced: %+v %v", got, err)
			}
		}
	}
}
