package telemetry

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"ocp.local/platform/internal/capability"
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
		case "completed", "retry", "lease_or_storage_failure":
		default:
			outcome = "_OTHER"
		}
		labels := append(attrs, attribute.String("ocp.outcome", outcome))
		span.SetAttributes(labels...)
		o.effects.Add(ctx, 1, metric.WithAttributes(labels...))
		o.effectDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(labels...))
		span.End()
	}
}
