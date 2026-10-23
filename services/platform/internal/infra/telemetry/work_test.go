package telemetry

import (
	"context"
	"encoding/json"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"ocp.local/platform/internal/core/capability"
	"strings"
	"testing"
	"time"
)

func TestEffectSignalsKeepBoundedLabels(t *testing.T) {
	o, exporter, reader, logs := fixture(t)
	_, done := o.StartEffect(context.Background(), capability.Food, "catalog.publish")
	done("completed")
	_, done = o.StartEffect(context.Background(), "private-merchant", "private-payload")
	done("private-result")
	spans := exporter.GetSpans()
	if len(spans) != 2 || attrs(spans[1].Attributes)["ocp.work.kind"] != "_OTHER" {
		t.Fatal("unbounded effect label")
	}
	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(struct {
		Spans   any
		Metrics any
		Logs    string
	}{spans, data, logs.String()})
	if strings.Contains(string(wire), "private-") {
		t.Fatal("private effect data escaped")
	}
	if !strings.Contains(string(wire), "ocp.work.attempts") {
		t.Fatal("missing work metric")
	}
}

func TestSchedulerSignalsAndSnapshotLabels(t *testing.T) {
	o, _, reader, logs := fixture(t)
	ctx := context.Background()
	for _, event := range []string{"listening", "listen_failure", "disconnected", "coalesced", "empty_claim", "claim_failure", "schedule_failure", "snapshot_failure", "private-payload"} {
		o.WorkEvent(ctx, event)
	}
	o.WorkClaim(ctx, true)
	o.WorkRetry(ctx, 2*time.Second)
	o.WorkBacklog(ctx, capability.Food, "pending", 7, 42)
	o.WorkBacklog(ctx, "private-merchant", "private-state", 1, 0)
	_, done := o.StartEffect(ctx, capability.Food, "catalog.publish")
	done("failed")
	var data metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &data); err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(data)
	if strings.Contains(string(wire)+logs.String(), "private-") {
		t.Fatal("private scheduler label leaked")
	}
	for _, name := range []string{"ocp.work.scheduler.events", "ocp.work.claimed", "ocp.work.retry.delay", "ocp.work.backlog", "ocp.work.pending.age", "failed", "_OTHER"} {
		if !strings.Contains(string(wire), name) {
			t.Fatalf("missing signal %s", name)
		}
	}
	// Nil observers preserve a cheap disabled path without forcing callers to branch.
	var disabled *Observer
	disabled.WorkEvent(ctx, "shutdown")
	disabled.WorkClaim(ctx, false)
	disabled.WorkRetry(ctx, time.Second)
	disabled.WorkBacklog(ctx, capability.Food, "pending", 0, 0)
}
