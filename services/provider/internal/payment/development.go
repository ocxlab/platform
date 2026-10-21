// Package payment owns financial-operation state, independently of Food's
// acceptance and fulfilment decisions. Development never contacts a real PSP.
package payment

import "errors"

var ErrTransition = errors.New("payment action unavailable")

const MaxAttempts = 10

type Operation struct {
	State    string
	Attempts int
}

// DevelopmentTransition is an explicit simulation control, not evidence from
// Stripe or another processor. The caller must authorize it and persist it.
func DevelopmentTransition(current Operation, action string) (Operation, error) {
	next := current
	switch action {
	case "start":
		if (current.State != "not_started" && current.State != "failed") || current.Attempts >= MaxAttempts {
			return current, ErrTransition
		}
		next.State = "awaiting_customer"
		next.Attempts++
	case "succeed", "decline", "abandon":
		if current.State != "awaiting_customer" {
			return current, ErrTransition
		}
		switch action {
		case "succeed":
			next.State = "paid"
		case "decline":
			next.State = "failed"
		case "abandon":
			next.State = "not_started"
		}
	case "cancel_order":
		switch current.State {
		case "paid":
			next.State = "refunded"
		case "awaiting_customer":
			next.State = "not_started"
		case "not_started", "failed":
		default:
			return current, ErrTransition
		}
	default:
		return current, ErrTransition
	}
	return next, nil
}
