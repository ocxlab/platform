package work

import (
	"context"
	"encoding/json"
	"time"

	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/persistence/sqlgen"
	"ocp.local/platform/internal/core/workmodel"
)

// StatsSource is optional observability, never a prerequisite for durable work.
type StatsSource interface {
	Snapshot(context.Context, []capability.Domain) ([]workmodel.Stats, error)
}

func (s SQLStore) Snapshot(ctx context.Context, domains []capability.Domain) ([]workmodel.Stats, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	names := make([]string, len(domains))
	for i, domain := range domains {
		names[i] = string(domain)
	}
	raw, err := json.Marshal(names)
	if err != nil {
		return nil, err
	}
	rows, err := sqlgen.New(s.DB).WorkSnapshot(ctx, raw)
	if err != nil {
		return nil, err
	}
	result := make([]workmodel.Stats, 0, len(rows))
	for _, row := range rows {
		result = append(result, workmodel.Stats{Domain: capability.Domain(row.Domain), State: row.State, Count: row.Amount, OldestSeconds: row.OldestSeconds})
	}
	return result, nil
}

func (w Worker) recordStats(ctx context.Context) {
	rows, err := w.Stats.Snapshot(ctx, w.Domains)
	if err != nil {
		w.Observer.WorkEvent(ctx, "snapshot_failure")
		return
	}
	for _, row := range rows {
		w.Observer.WorkBacklog(ctx, row.Domain, row.State, row.Count, row.OldestSeconds)
	}
}
