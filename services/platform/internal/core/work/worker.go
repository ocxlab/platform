// Package work delivers committed effect intents. A lease coordinates attempts;
// adapters still need stable effect identities and idempotent reconciliation.
package work

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/infra/telemetry"
)

type Handler func(context.Context, persistence.Effect) error
type Worker struct {
	DB          *sql.DB
	Domains     []capability.Domain
	Handlers    map[string]Handler
	Concurrency int
	Observer    *telemetry.Observer
}

// Once claims only as many effects as it can start immediately, keeping adapter
// calls outside SQL transactions. Each attempt is shorter than its lease.
func (w Worker) Once(ctx context.Context) (int, error) {
	if w.Concurrency < 1 || w.Concurrency > 20 || len(w.Domains) == 0 || len(w.Handlers) == 0 {
		return 0, errors.New("invalid worker configuration")
	}
	for _, domain := range w.Domains {
		// Every claimable kind must have an implementation in this process. Grow
		// this list alongside the persistence registry, never drop unknown work.
		if !persistence.SupportedEffect(domain, "catalog.publish") || w.Handlers[string(domain)+"/catalog.publish"] == nil {
			return 0, errors.New("domain has no installed work handlers")
		}
	}
	effects, err := persistence.ClaimEffects(ctx, w.DB, w.Domains, w.Concurrency, 30*time.Second)
	if err != nil {
		return 0, err
	}
	results := make(chan error, len(effects))
	for _, effect := range effects {
		go func(e persistence.Effect) {
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
			finish, cancelFinish := context.WithTimeout(ctx, 3*time.Second)
			defer cancelFinish()
			done, err := persistence.FinishEffect(finish, w.DB, e, attempt == nil, retry)
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
func (w Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, err := w.Once(ctx); err != nil && ctx.Err() == nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
