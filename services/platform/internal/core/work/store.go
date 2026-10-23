package work

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/persistence"
	"ocp.local/platform/internal/core/persistence/sqlgen"
	"ocp.local/platform/internal/core/workmodel"
)

// Store is the worker-facing contract. Decision/enqueue remains in its owning
// transaction; no transaction handles or generated rows cross this interface.
type Store interface {
	Claim(context.Context, []capability.Domain, int, time.Duration) ([]workmodel.Effect, error)
	Finish(context.Context, workmodel.Effect, bool, time.Duration) (bool, error)
	NextDelay(context.Context, []capability.Domain) (*time.Duration, error)
}

type SQLStore struct{ DB *sql.DB }

func (s SQLStore) Claim(ctx context.Context, domains []capability.Domain, batch int, lease time.Duration) ([]workmodel.Effect, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return persistence.ClaimEffects(ctx, s.DB, domains, batch, lease)
}
func (s SQLStore) Finish(ctx context.Context, e workmodel.Effect, success bool, retry time.Duration) (bool, error) {
	return persistence.FinishEffect(ctx, s.DB, e, success, retry)
}
func (s SQLStore) NextDelay(ctx context.Context, domains []capability.Domain) (*time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	names := make([]string, 0, len(domains))
	for _, domain := range domains {
		names = append(names, string(domain))
	}
	raw, err := json.Marshal(names)
	if err != nil {
		return nil, err
	}
	seconds, err := sqlgen.New(s.DB).NextEffectDelay(ctx, raw)
	if err != nil {
		return nil, err
	}
	if seconds < 0 {
		return nil, nil
	}
	// Runtime retries/leases are much shorter; cap before converting a timestamp
	// manually changed by an administrator into an overflowing Go duration.
	seconds = min(seconds, 3600)
	delay := time.Duration(seconds * float64(time.Second))
	return &delay, nil
}
