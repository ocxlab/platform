package agenttools

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	commerce "ocp.local/platform/internal/core/application"
	"ocp.local/platform/internal/core/persistence"
	foodapp "ocp.local/platform/internal/food/application"
	foodstore "ocp.local/platform/internal/food/persistence"
)

type orderListInput struct {
	Store string `json:"store"`
	After string `json:"after,omitempty" jsonschema:"Opaque next_after from the previous page; omit to refresh newest orders"`
}
type orderReadInput struct {
	Store            string `json:"store"`
	Order            string `json:"order" jsonschema:"Exact order handle returned by list_food_orders; do not infer it from a receipt"`
	ExpectedRevision int64  `json:"expected_revision,omitempty" jsonschema:"Optional operational revision guard; required for item-page continuation"`
	Offset           int    `json:"offset,omitempty"`
	History          bool   `json:"history,omitempty" jsonschema:"Include bounded operational history only when needed"`
}
type orderDecisionInput struct {
	Store            string `json:"store"`
	Order            string `json:"order"`
	ExpectedRevision int64  `json:"expected_revision"`
	Decision         string `json:"decision" jsonschema:"accept or reject; explicit merchant decision, never inferred from a menu edit"`
	RequestKey       string `json:"request_key"`
}
type pickupInput struct {
	Store            string `json:"store"`
	Order            string `json:"order"`
	ExpectedRevision int64  `json:"expected_revision"`
	Action           string `json:"action" jsonschema:"start_preparation, mark_ready, or record_collection; report actual work, never skip stages"`
	RequestKey       string `json:"request_key"`
}

func orderSummary(order foodstore.FoodOrder) map[string]any {
	return map[string]any{"order": order.ID.String(), "reference": order.Reference, "revision": order.Revision, "created_at": order.CreatedAt, "acceptance": order.Acceptance, "payment": order.Payment, "payment_provider": order.PaymentProvider, "fulfilment": order.Fulfilment, "fulfilment_progress": order.FulfilmentProgress, "resolution": order.Resolution, "currency": order.Terms.Currency, "total_minor": order.Terms.Total}
}
func orderDetails(order foodstore.FoodOrder, in orderReadInput) (map[string]any, error) {
	if in.Offset < 0 || in.ExpectedRevision < 0 || (in.Offset > 0 && in.ExpectedRevision == 0) {
		return nil, commerce.ErrInvalid
	}
	if in.ExpectedRevision > 0 && in.ExpectedRevision != order.Revision {
		return nil, &foodstore.OrderRevisionConflict{Current: order.Revision}
	}
	var lines []json.RawMessage
	if json.Unmarshal(order.Terms.Lines, &lines) != nil {
		return nil, commerce.ErrInvalid
	}
	if in.Offset > len(lines) {
		return nil, commerce.ErrInvalid
	}
	end := min(in.Offset+20, len(lines))
	out := orderSummary(order)
	out["items"], out["total_items"], out["note"] = lines[in.Offset:end], len(lines), order.Terms.Note
	if end < len(lines) {
		out["next_offset"] = end
	}
	if in.History {
		out["history"] = order.History
	}
	return out, nil
}

func (s Merchant) addOrderTools(server *mcp.Server, p persistence.Principal) {
	if !p.AuthorizesMerchant(p.MerchantID, "order:read") {
		return
	}
	closed := false
	orders := foodapp.Orders{DB: s.DB}
	mcp.AddTool(server, &mcp.Tool{Name: "list_food_orders", Description: "Read the newest 20 merchant Food orders with human receipt references, operational revisions, states and totals. No customer credentials, full order items or history. Use inspect_food_order for selected orders. Pagination is live; omit after to refresh.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}}, func(ctx context.Context, _ *mcp.CallToolRequest, in orderListInput) (*mcp.CallToolResult, map[string]any, error) {
		store, err := s.resolve(ctx, p, in.Store)
		if err != nil {
			return toolError(err)
		}
		var after *string
		if in.After != "" {
			after = &in.After
		}
		page, err := orders.List(ctx, p, store, after, 20)
		if err != nil {
			return toolError(err)
		}
		items := make([]map[string]any, 0, len(page.Items))
		for _, order := range page.Items {
			items = append(items, orderSummary(order))
		}
		out := map[string]any{"store": in.Store, "orders": items}
		if page.NextAfter != nil {
			out["next_after"] = *page.NextAfter
		}
		return nil, out, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "inspect_food_order", Description: "Inspect one owned Food order: agreed item names/options/notes and totals, up to 20 lines. Optional history adds operational events. Keep returned revision for continuation and mutations; stale revisions require a fresh read. Does not authorize payment or change orders.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}}, func(ctx context.Context, _ *mcp.CallToolRequest, in orderReadInput) (*mcp.CallToolResult, map[string]any, error) {
		id, err := uuid.Parse(in.Order)
		if err != nil {
			return toolError(commerce.ErrInvalid)
		}
		store, err := s.resolve(ctx, p, in.Store)
		if err != nil {
			return toolError(err)
		}
		order, err := orders.Get(ctx, p, store, id)
		if err != nil {
			return toolError(err)
		}
		out, err := orderDetails(order, in)
		if err != nil {
			return toolError(err)
		}
		return nil, out, nil
	})
	if !p.AuthorizesMerchant(p.MerchantID, "order:manage") {
		return
	}
	mutate := func(ctx context.Context, storeHandle, orderHandle string, revision int64, action, key string) (*mcp.CallToolResult, map[string]any, error) {
		id, err := uuid.Parse(orderHandle)
		if err != nil {
			return toolError(commerce.ErrInvalid)
		}
		store, err := s.resolve(ctx, p, storeHandle)
		if err != nil {
			return toolError(err)
		}
		result, err := orders.Change(ctx, p, store, id, revision, action, key, uuid.New())
		if err != nil {
			return toolError(err)
		}
		var order foodstore.FoodOrder
		if err = json.Unmarshal(result.Body, &order); err != nil {
			return toolError(err)
		}
		out := orderSummary(order)
		out["store"], out["replayed"] = storeHandle, result.Replayed
		return nil, out, nil
	}
	mcp.AddTool(server, &mcp.Tool{Name: "decide_food_order", Description: "Explicitly accept or reject one pending merchant Food order using its inspected operational revision and a retry key. Neither decision charges money nor starts preparation. Rejection closes the order; retries retain historical results.", Annotations: &mcp.ToolAnnotations{IdempotentHint: true, OpenWorldHint: &closed}}, func(ctx context.Context, _ *mcp.CallToolRequest, in orderDecisionInput) (*mcp.CallToolResult, map[string]any, error) {
		if in.Decision != "accept" && in.Decision != "reject" {
			return toolError(commerce.ErrInvalid)
		}
		return mutate(ctx, in.Store, in.Order, in.ExpectedRevision, in.Decision, in.RequestKey)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "advance_pickup_order", Description: "Report actual preparation, ready-for-pickup, or collection for an accepted, paid, open pickup order. Stages cannot be skipped or reversed. Requires inspected operational revision and retry key. This reports work; it does not dispatch a printer or collect payment.", Annotations: &mcp.ToolAnnotations{IdempotentHint: true, OpenWorldHint: &closed}}, func(ctx context.Context, _ *mcp.CallToolRequest, in pickupInput) (*mcp.CallToolResult, map[string]any, error) {
		switch in.Action {
		case "start_preparation", "mark_ready", "record_collection":
		default:
			return toolError(commerce.ErrInvalid)
		}
		return mutate(ctx, in.Store, in.Order, in.ExpectedRevision, in.Action, in.RequestKey)
	})
}
