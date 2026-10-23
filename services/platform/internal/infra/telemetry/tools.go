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
	return o.StartTool(ctx, "merchant", name)
}

func (o *Observer) StartTool(ctx context.Context, surface, name string) (context.Context, func(string)) {
	if surface != "merchant" && surface != "customer-food" && surface != "core-account" {
		surface = "_OTHER"
		name = "_OTHER"
	}
	switch name {
	case "inspect_my_profile", "update_my_preferences":
	case "preview_menu_publication", "publish_menu_revision":
	case "list_food_orders", "inspect_food_order", "decide_food_order", "advance_pickup_order":
	case "find_food_stores", "inspect_store_menu", "inspect_food_dish", "inspect_food_meals", "price_food_basket", "inspect_food_basket", "revise_food_basket", "commit_food_order", "track_my_food_order", "cancel_my_food_order":
	case "find_my_stores", "inspect_menu", "inspect_menu_item", "inspect_item_extras", "set_item_availability", "update_item_prices", "review_menu_changes", "inspect_meal_offers", "save_meal_offer", "inspect_menu_structure", "save_menu_item", "organize_menu_categories":
	default:
		name = "_OTHER"
	}
	attrs := []attribute.KeyValue{attribute.String("ocp.mcp.surface", surface), attribute.String("ocp.mcp.tool", name)}
	ctx, span := o.tracer.Start(ctx, "mcp "+name, trace.WithAttributes(attrs...))
	start := time.Now()
	o.toolActive.Add(ctx, 1, metric.WithAttributes(attrs...))
	return ctx, func(code string) {
		outcome := "success"
		switch code {
		case "":
		case "account_revision_conflict":
			outcome = "rejected"
		case "forbidden":
			outcome = "forbidden"
		case "invalid_request", "not_found", "revision_conflict", "publication_conflict", "order_revision_conflict", "order_transition_conflict", "idempotency_conflict", "unsupported_model", "invalid_offering", "unauthenticated", "invalid_selection", "quote_expired", "quote_already_placed", "ordering_paused", "quote_capacity", "guest_capacity":
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
			o.logger.InfoContext(ctx, "MCP tool completed", "surface", surface, "tool", name, "outcome", outcome, "code", code, "trace_id", span.SpanContext().TraceID().String(), "span_id", span.SpanContext().SpanID().String())
		}
		span.End()
	}
}
