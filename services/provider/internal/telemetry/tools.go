package telemetry

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"time"
)

// StartMerchantTool accepts only registered tool/code labels, never arguments,
// results, merchant identifiers, or arbitrary client-supplied tool names.
func (o *Observer) StartMerchantTool(ctx context.Context, name string) (context.Context, func(string)) {
	switch name {
	case "find_my_stores", "inspect_menu", "inspect_menu_item", "inspect_item_extras", "set_item_availability", "update_item_prices", "review_menu_changes":
	default:
		name = "_OTHER"
	}
	attrs := []attribute.KeyValue{attribute.String("ocp.mcp.surface", "merchant"), attribute.String("ocp.mcp.tool", name)}
	ctx, span := o.tracer.Start(ctx, "mcp "+name, trace.WithAttributes(attrs...))
	start := time.Now()
	o.toolActive.Add(ctx, 1, metric.WithAttributes(attrs...))
	return ctx, func(code string) {
		outcome := "success"
		switch code {
		case "":
		case "forbidden":
			outcome = "forbidden"
		case "invalid_request", "not_found", "revision_conflict", "idempotency_conflict", "unsupported_model", "invalid_offering":
			outcome = "rejected"
		case "timeout":
			outcome = "timeout"
		case "cancelled":
			outcome = "cancelled"
		default:
			code = "unavailable"
			outcome = "internal_error"
		}
		finished := append(append([]attribute.KeyValue{}, attrs...), attribute.String("ocp.outcome", outcome))
		if code != "" {
			finished = append(finished, attribute.String("ocp.error.code", code))
		}
		span.SetAttributes(finished...)
		if outcome == "internal_error" || outcome == "timeout" {
			span.SetStatus(codes.Error, outcome)
		}
		o.toolActive.Add(ctx, -1, metric.WithAttributes(attrs...))
		o.toolCalls.Add(ctx, 1, metric.WithAttributes(finished...))
		o.toolDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(finished...))
		if code != "" && o.logger != nil {
			o.logger.InfoContext(ctx, "MCP tool completed", "surface", "merchant", "tool", name, "outcome", outcome, "code", code, "trace_id", span.SpanContext().TraceID().String(), "span_id", span.SpanContext().SpanID().String())
		}
		span.End()
	}
}
