package agenttools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	commerce "ocp.local/platform/internal/core/application"
	"ocp.local/platform/internal/core/offering"
	"ocp.local/platform/internal/core/persistence"
	food "ocp.local/platform/internal/food/offering"
)

type publicationPreviewInput struct {
	Store              string `json:"store"`
	Revision           int64  `json:"revision" jsonschema:"Exact positive retained Food revision to publish; never implicit latest"`
	ExpectedGeneration *int64 `json:"expected_generation,omitempty" jsonschema:"Use returned generation on continuation; a changed publication requires a new review"`
	Offset             int    `json:"offset,omitempty"`
}
type publishMenuInput struct {
	Store              string `json:"store"`
	Revision           int64  `json:"revision"`
	ExpectedGeneration int64  `json:"expected_generation"`
	ReviewToken        string `json:"review_token" jsonschema:"Token from preview_menu_publication for this exact store, revision and generation"`
	Confirmed          bool   `json:"confirmed" jsonschema:"True only after explicit merchant instruction to publish the reviewed revision; preview alone is not consent"`
	RequestKey         string `json:"request_key"`
}

// This digest binds selection, not authority or human consent. Resource-scoped
// authorization and the shared publication transaction remain authoritative.
func publicationReviewToken(store uuid.UUID, revision, generation int64, document json.RawMessage) string {
	data, _ := json.Marshal(struct {
		Store                uuid.UUID
		Revision, Generation int64
		Document             json.RawMessage
	}{store, revision, generation, document})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s Merchant) addPublicationTools(server *mcp.Server, p persistence.Principal) {
	if !p.AuthorizesMerchant(p.MerchantID, "offering:read") {
		return
	}
	closed := false
	mcp.AddTool(server, &mcp.Tool{Name: "preview_menu_publication", Description: "Review an exact draft against the current published menu (or empty menu for first publication). Returns up to 20 field changes, publication generation, suspension flag and a selection-binding review token. Keep revision/generation on continuation. Review all pages and obtain explicit merchant consent before publish_menu_revision. A token is not proof of human approval.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}}, func(ctx context.Context, _ *mcp.CallToolRequest, in publicationPreviewInput) (*mcp.CallToolResult, map[string]any, error) {
		if in.Revision < 1 || in.Offset < 0 || (in.Offset > 0 && in.ExpectedGeneration == nil) || (in.ExpectedGeneration != nil && *in.ExpectedGeneration < 0) {
			return toolError(commerce.ErrInvalid)
		}
		store, err := s.resolve(ctx, p, in.Store)
		if err != nil {
			return toolError(err)
		}
		current, err := persistence.GetPublication(ctx, s.DB, p.MerchantID, store)
		if err != nil {
			return toolError(err)
		}
		if in.ExpectedGeneration != nil && *in.ExpectedGeneration != current.Generation {
			return toolError(&persistence.PublicationConflict{Current: current.Generation})
		}
		target, err := s.menuBase(ctx, p, in.Store, in.Revision)
		if err != nil {
			return toolError(err)
		}
		baseline := json.RawMessage(`{}`)
		if current.Revision != nil {
			source, err := persistence.GetOffering(ctx, s.DB, p.MerchantID, store, *current.Revision)
			if err != nil {
				return toolError(err)
			}
			if source.Model != food.Model {
				return toolError(offering.ErrUnsupported)
			}
			baseline = source.Document
		}
		changes, err := menuChanges(baseline, target.Document)
		if err != nil {
			return toolError(err)
		}
		if in.Offset > len(changes) {
			return toolError(commerce.ErrInvalid)
		}
		end := min(in.Offset+20, len(changes))
		out := map[string]any{"store": in.Store, "revision": in.Revision, "generation": current.Generation, "published_revision": current.Revision, "suspended": current.Suspended, "changes": changes[in.Offset:end], "total_changes": len(changes), "review_token": publicationReviewToken(store, in.Revision, current.Generation, target.Document), "approval_required": true}
		if end < len(changes) {
			out["next_offset"] = end
		}
		return nil, out, nil
	})
	if !p.AuthorizesMerchant(p.MerchantID, "offering:write") {
		return
	}
	mcp.AddTool(server, &mcp.Tool{Name: "publish_menu_revision", Description: "Make one explicitly reviewed Food revision public after merchant consent. Requires matching preview token, generation, confirmed=true and retry key. Never substitutes a newer draft or removes provider suspension. A retained successful retry returns its original publication, not necessarily current visibility.", Annotations: &mcp.ToolAnnotations{IdempotentHint: true, OpenWorldHint: &closed}}, func(ctx context.Context, _ *mcp.CallToolRequest, in publishMenuInput) (*mcp.CallToolResult, map[string]any, error) {
		if !in.Confirmed || in.Revision < 1 || in.ExpectedGeneration < 0 {
			return toolError(commerce.ErrInvalid)
		}
		store, err := s.resolve(ctx, p, in.Store)
		if err != nil {
			return toolError(err)
		}
		target, err := s.menuBase(ctx, p, in.Store, in.Revision)
		if err != nil {
			return toolError(err)
		}
		if in.ReviewToken != publicationReviewToken(store, in.Revision, in.ExpectedGeneration, target.Document) {
			return toolError(commerce.ErrInvalid)
		}
		result, err := (commerce.Publications{DB: s.DB, Models: s.Models}).Change(ctx, p, p.MerchantID, store, in.ExpectedGeneration, &in.Revision, in.RequestKey, uuid.New())
		if err != nil {
			return toolError(err)
		}
		var published persistence.Publication
		if err = json.Unmarshal(result.Body, &published); err != nil {
			return toolError(err)
		}
		return nil, map[string]any{"store": in.Store, "published_revision": published.Revision, "generation": published.Generation, "suspended": published.Suspended, "replayed": result.Replayed}, nil
	})
}
