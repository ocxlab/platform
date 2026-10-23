package telemetry

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"ocp.local/platform/internal/core/capability"
	"time"
)

// StartEffect accepts only installed, bounded labels; no effect IDs or payloads.
func (o *Observer) StartEffect(ctx context.Context, domain capability.Domain, kind string) (context.Context, func(string)) {
	d, k := "_OTHER", "_OTHER"
	if domain == capability.Food && kind == "catalog.publish" {
		d, k = string(domain), kind
	}
	attrs := []attribute.KeyValue{attribute.String("ocp.domain", d), attribute.String("ocp.work.kind", k)}
	ctx, span := o.tracer.Start(ctx, "work "+k, trace.WithAttributes(attrs...))
	start := time.Now()
	return ctx, func(outcome string) {
		switch outcome {
		case "completed", "retry", "failed", "lease_or_storage_failure":
		default:
			outcome = "_OTHER"
		}
		labels := append(attrs, attribute.String("ocp.outcome", outcome))
		span.SetAttributes(labels...)
		if outcome != "completed" {
			span.SetStatus(codes.Error, outcome)
		}
		o.effects.Add(ctx, 1, metric.WithAttributes(labels...))
		o.effectDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(labels...))
		span.End()
	}
}

// WorkEvent deliberately accepts a closed vocabulary, never driver errors or hints.
func (o *Observer) WorkEvent(ctx context.Context, event string) {
	if o == nil {
		return
	}
	switch event {
	case "listen_attempt", "listening", "listen_failure", "disconnected", "notification", "ignored_notification", "coalesced", "drain_wake", "drain_timer", "empty_claim", "claim_failure", "schedule_failure", "snapshot_failure", "shutdown", "wake_stopped":
	default:
		event = "_OTHER"
	}
	o.workEvents.Add(ctx, 1, metric.WithAttributes(attribute.String("ocp.work.event", event)))
}

func (o *Observer) WorkClaim(ctx context.Context, repeated bool) {
	if o == nil {
		return
	}
	o.workClaimed.Add(ctx, 1, metric.WithAttributes(attribute.Bool("ocp.work.repeated", repeated)))
}

func (o *Observer) WorkRetry(ctx context.Context, delay time.Duration) {
	if o != nil {
		o.workRetry.Record(ctx, max(0, delay.Seconds()))
	}
}

// Backlog gauges are database totals sampled by each worker, not additive counters.
func (o *Observer) WorkBacklog(ctx context.Context, domain capability.Domain, state string, count int64, ageSeconds float64) {
	if o == nil {
		return
	}
	d := "_OTHER"
	if domain.Valid() {
		d = string(domain)
	}
	switch state {
	case "pending", "delayed", "leased", "expired", "failed":
	default:
		state = "_OTHER"
	}
	labels := metric.WithAttributes(attribute.String("ocp.domain", d), attribute.String("ocp.work.state", state))
	o.workBacklog.Record(ctx, max(0, count), labels)
	if state == "pending" {
		o.workAge.Record(ctx, max(0, ageSeconds), metric.WithAttributes(attribute.String("ocp.domain", d)))
	}
}
