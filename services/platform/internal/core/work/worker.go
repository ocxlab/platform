// Package work delivers committed effect intents. A lease coordinates attempts;
// adapters still need stable effect identities and idempotent reconciliation.
package work

import (
	"context"
	"errors"
	"time"

	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/core/workmodel"
	"ocp.local/platform/internal/infra/telemetry"
)

type Handler func(context.Context, workmodel.Effect) error
type Worker struct {
	Store          Store
	Wake           WakeSource
	SafetyInterval time.Duration
	Domains        []capability.Domain
	Handlers       map[string]Handler
	Concurrency    int
	Observer       *telemetry.Observer
}

func (w Worker) validate() error {
	if w.Store == nil || w.Concurrency < 1 || w.Concurrency > 20 || len(w.Domains) == 0 || len(w.Handlers) == 0 {
		return errors.New("invalid worker configuration")
	}
	for _, domain := range w.Domains {
		// Every claimable kind must have an implementation in this process. Grow
		// this list alongside the persistence registry, never drop unknown work.
		if !persistence.SupportedEffect(domain, "catalog.publish") || w.Handlers[string(domain)+"/catalog.publish"] == nil {
			return errors.New("domain has no installed work handlers")
		}
	}
	return nil
}

// Once claims only as many effects as it can start immediately, keeping adapter
// calls outside SQL transactions. Each attempt is shorter than its lease.
func (w Worker) Once(ctx context.Context) (int, error) {
	if err := w.validate(); err != nil {
		return 0, err
	}
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	effects, err := w.Store.Claim(ctx, w.Domains, w.Concurrency, 30*time.Second)
	if err != nil {
		return 0, err
	}
	results := make(chan error, len(effects))
	for _, effect := range effects {
		go func(e workmodel.Effect) {
			attemptCtx := ctx
			finishSignal := func(string) {}
			if w.Observer != nil {
				attemptCtx, finishSignal = w.Observer.StartEffect(ctx, e.Domain, e.Kind)
			}
			handler, ok := w.Handlers[string(e.Domain)+"/"+e.Kind]
			call, cancel := context.WithTimeout(attemptCtx, 10*time.Second)
			defer cancel()
			var attempt error
			if !ok {
				attempt = errors.New("unsupported effect handler")
			} else {
				attempt = handler(call, e)
			}
			retry := time.Duration(1<<min(e.Attempts, 10)) * time.Second
			if !ok {
				retry = 0
			}
			// Bounded cleanup survives shutdown cancellation. The handler remains
			// canceled; an incomplete or ambiguous external attempt is retried.
			finish, cancelFinish := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancelFinish()
			done, err := w.Store.Finish(finish, e, attempt == nil, retry)
			if err == nil && !done {
				err = errors.New("effect lease lost")
			}
			outcome := "completed"
			if attempt != nil {
				outcome = "retry"
			}
			if err != nil {
				outcome = "lease_or_storage_failure"
			}
			finishSignal(outcome)
			results <- err
		}(effect)
	}
	for range effects {
		if e := <-results; e != nil {
			err = e
		}
	}
	return len(effects), err
}
