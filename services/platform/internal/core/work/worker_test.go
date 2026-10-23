package work

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace/noop"
	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/workmodel"
	"ocp.local/platform/internal/infra/telemetry"
)

type attemptStore struct {
	fakeStore
	effect workmodel.Effect
	lease  time.Duration
}

func (s *attemptStore) Claim(_ context.Context, domains []capability.Domain, batch int, lease time.Duration) ([]workmodel.Effect, error) {
	if len(domains) != 1 || domains[0] != capability.Food || batch != 2 {
		return nil, errors.New("wrong claim scope or bound")
	}
	s.lease = lease
	return []workmodel.Effect{s.effect}, nil
}

func TestAttemptOutcomesAndLeaseBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		attempts                             int
		kind                                 string
		handlerFails, finishFails, leaseLost bool
		outcome                              string
	}{
		{name: "success", attempts: 1, outcome: "completed"},
		{name: "retry", attempts: 2, handlerFails: true, outcome: "retry"},
		{name: "last-retry", attempts: 19, handlerFails: true, outcome: "retry"},
		{name: "exhausted", attempts: 20, handlerFails: true, outcome: "failed"},
		{name: "unknown-kind", attempts: 1, kind: "unknown", outcome: "failed"},
		{name: "storage-failure", attempts: 1, finishFails: true, outcome: "lease_or_storage_failure"},
		{name: "lease-lost", attempts: 1, leaseLost: true, outcome: "lease_or_storage_failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := sdkmetric.NewManualReader()
			mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			defer func() { _ = mp.Shutdown(context.Background()) }()
			observer, err := telemetry.NewObserver(noop.NewTracerProvider(), mp, slog.Default())
			if err != nil {
				t.Fatal(err)
			}
			kind := tc.kind
			if kind == "" {
				kind = "catalog.publish"
			}
			store := &attemptStore{effect: workmodel.Effect{ID: uuid.New(), Domain: capability.Food, Kind: kind, Attempts: tc.attempts}}
			finished := false
			store.finish = func(ctx context.Context, e workmodel.Effect, success bool, retry time.Duration) (bool, error) {
				finished = true
				deadline, bounded := ctx.Deadline()
				if !bounded || time.Until(deadline) < 2*time.Second || time.Until(deadline) > 3*time.Second {
					t.Error("cleanup deadline must be three seconds")
				}
				wantRetry := time.Duration(1<<min(tc.attempts, 10)) * time.Second
				if tc.kind != "" {
					wantRetry = 0
				}
				if e.ID != store.effect.ID || success != (!tc.handlerFails && tc.kind == "") || retry != wantRetry {
					t.Errorf("wrong finish identity/success/retry: %v %v %s", e.ID, success, retry)
				}
				if tc.finishFails {
					return false, errors.New("storage unavailable")
				}
				return !tc.leaseLost, nil
			}
			w := testWorker(store, nil)
			w.Observer = observer
			called := false
			w.Handlers["food/catalog.publish"] = func(ctx context.Context, e workmodel.Effect) error {
				called = true
				deadline, bounded := ctx.Deadline()
				if !bounded || time.Until(deadline) < 9*time.Second || time.Until(deadline) > 10*time.Second {
					t.Error("handler deadline must be ten seconds")
				}
				if e.ID != store.effect.ID {
					t.Error("handler identity changed")
				}
				if tc.handlerFails {
					return errors.New("retry me")
				}
				return nil
			}
			count, err := w.Once(context.Background())
			if count != 1 || !finished || called != (tc.kind == "") || (err != nil) != (tc.finishFails || tc.leaseLost) || store.lease != 30*time.Second {
				t.Fatalf("wrong attempt result: count=%d called=%v finished=%v lease=%s err=%v", count, called, finished, store.lease, err)
			}
			var data metricdata.ResourceMetrics
			if err := reader.Collect(context.Background(), &data); err != nil {
				t.Fatal(err)
			}
			var outcomes, claims, retries uint64
			for _, scope := range data.ScopeMetrics {
				for _, m := range scope.Metrics {
					switch m.Name {
					case "ocp.work.attempts":
						for _, point := range m.Data.(metricdata.Sum[int64]).DataPoints {
							outcome, _ := point.Attributes.Value(attribute.Key("ocp.outcome"))
							if outcome.AsString() != tc.outcome {
								t.Errorf("outcome=%s want=%s", outcome.AsString(), tc.outcome)
							}
							outcomes += uint64(point.Value)
						}
					case "ocp.work.claimed":
						for _, point := range m.Data.(metricdata.Sum[int64]).DataPoints {
							repeated, _ := point.Attributes.Value(attribute.Key("ocp.work.repeated"))
							if repeated.AsBool() != (tc.attempts > 1) {
								t.Error("wrong repeated-claim label")
							}
							claims += uint64(point.Value)
						}
					case "ocp.work.retry.delay":
						for _, point := range m.Data.(metricdata.Histogram[float64]).DataPoints {
							retries += point.Count
						}
					}
				}
			}
			wantRetries := uint64(0)
			if tc.outcome == "retry" {
				wantRetries = 1
			}
			if outcomes != 1 || claims != 1 || retries != wantRetries {
				t.Fatalf("metric counts: outcomes=%d claims=%d retries=%d", outcomes, claims, retries)
			}
		})
	}
}

func TestWorkerAcceptsConcurrencyLimits(t *testing.T) {
	for _, concurrency := range []int{1, 20} {
		w := testWorker(&fakeStore{}, nil)
		w.Concurrency = concurrency
		if err := w.validate(); err != nil {
			t.Fatalf("rejected valid boundary %d: %v", concurrency, err)
		}
	}
}

func TestEmptyClaimRecordsEmptyDrain(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()
	observer, err := telemetry.NewObserver(noop.NewTracerProvider(), mp, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	w := testWorker(&fakeStore{}, nil)
	w.Observer = observer
	if count, err := w.Once(context.Background()); count != 0 || err != nil {
		t.Fatalf("empty claim: %d %v", count, err)
	}
	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatal(err)
	}
	var empty int64
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == "ocp.work.scheduler.events" {
				for _, point := range m.Data.(metricdata.Sum[int64]).DataPoints {
					event, _ := point.Attributes.Value(attribute.Key("ocp.work.event"))
					if event.AsString() == "empty_claim" {
						empty += point.Value
					}
				}
			}
		}
	}
	if empty != 1 {
		t.Fatalf("empty-drain counter=%d want=1", empty)
	}
}
