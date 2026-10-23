package agenttools

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ocp.local/platform/internal/core/persistence"
)

func TestLocalBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, host, origin, method, auth string
		status                           int
	}{
		{"foreign_host", "attacker.invalid", "", "POST", "", 403},
		{"foreign_origin", "localhost:18180", "https://attacker.invalid", "POST", "", 403},
		{"null_origin", "localhost:18180", "null", "POST", "", 403},
		{"origin_credentials", "localhost:18180", "http://user@localhost:18180", "POST", "", 403},
		{"origin_path", "localhost:18180", "http://localhost:18180/a", "POST", "", 403},
		{"method", "localhost:18180", "", "GET", "", 405},
		{"missing", "localhost:18180", "", "POST", "", 401},
		{"invalid", "localhost:18180", "http://localhost:18180", "POST", "Bearer not-a-token", 401},
		{"ipv6", "[::1]:18180", "", "POST", "", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://localhost:18180/mcp/merchant", strings.NewReader(`{}`))
			r.Host = tc.host
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Authorization", tc.auth)
			w := httptest.NewRecorder()
			(Merchant{}).LocalHandler().ServeHTTP(w, r)
			if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.status == 401 && w.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing challenge")
			}
			if _, err := uuid.Parse(w.Header().Get("X-Request-ID")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestToolDiscoveryAndSchema(t *testing.T) {
	for _, tc := range []struct {
		name   string
		scopes []string
		count  int
	}{
		{"none", nil, 0}, {"store", []string{"store:read"}, 1}, {"menu", []string{"offering:read"}, 7},
		{"write_without_read", []string{"offering:write"}, 0},
		{"full", []string{"store:read", "offering:read", "offering:write"}, 14},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			a, b := mcp.NewInMemoryTransports()
			p := persistence.Principal{ID: uuid.New(), MerchantID: uuid.New(), Scopes: tc.scopes}
			serverSession, err := (Merchant{}).server(p).Connect(ctx, a, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer serverSession.Close()
			client, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, b, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			tools, err := client.ListTools(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(tools.Tools) != tc.count {
				t.Fatalf("tools=%v", tools.Tools)
			}
			for _, tool := range tools.Tools {
				if tool.InputSchema == nil || tool.Description == "" {
					t.Fatal("missing contract")
				}
				if tool.Name == "save_meal_offer" {
					wire, err := json.Marshal(tool.InputSchema)
					if err != nil {
						t.Fatal(err)
					}
					var schema struct {
						Properties map[string]struct {
							Properties map[string]struct {
								Type               any
								MinItems, MaxItems int
								UniqueItems        bool
							}
						}
					}
					if err := json.Unmarshal(wire, &schema); err != nil {
						t.Fatal(err)
					}
					for _, field := range []string{"main_item_ids", "addon_choices"} {
						array := schema.Properties["offer"].Properties[field]
						if array.Type != "array" || array.MinItems != 1 || array.MaxItems != 200 || !array.UniqueItems {
							t.Fatalf("invalid offer schema: %s", wire)
						}
					}
					if schema.Properties["offer"].Properties["translations"].Type != "object" {
						t.Fatal("translations must reject null")
					}
				}
				if tool.Name == "inspect_item_extras" || tool.Name == "update_item_prices" {
					field := "groups"
					if tool.Name == "update_item_prices" {
						field = "changes"
					}
					data, err := json.Marshal(tool.InputSchema)
					if err != nil {
						t.Fatal(err)
					}
					var schema struct {
						Properties map[string]struct {
							Type any `json:"type"`
						} `json:"properties"`
					}
					if json.Unmarshal(data, &schema) != nil || schema.Properties[field].Type != "array" {
						t.Fatalf("extras must advertise a non-null array: %s", data)
					}
				}
			}
			// Discovery is not authorization: calling an undisclosed tool is denied.
			if tc.count == 0 {
				result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "set_item_availability", Arguments: map[string]any{}})
				if err == nil && !result.IsError {
					t.Fatal("undisclosed mutation accepted")
				}
			}
			if tc.name == "menu" || tc.name == "write_without_read" {
				result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "update_item_prices", Arguments: map[string]any{}})
				if err == nil && !result.IsError {
					t.Fatal("price editing without read/write authority")
				}
			}
		})
	}
}

func TestToolErrorsAreSafe(t *testing.T) {
	result, _, err := toolError(&persistence.RevisionConflict{Current: 7})
	if err != nil || !result.IsError {
		t.Fatal("missing tool error")
	}
	var out map[string]any
	if json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &out) != nil || out["code"] != "revision_conflict" || out["current_revision"] != float64(7) {
		t.Fatal(out)
	}
}

func TestToolWorkHasDeadlineWithoutTelemetry(t *testing.T) {
	ctx := context.Background()
	a, b := mcp.NewInMemoryTransports()
	server := (Merchant{}).server(persistence.Principal{})
	mcp.AddTool(server, &mcp.Tool{Name: "deadline_probe", Description: "Test only"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, map[string]any, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 10*time.Second || time.Until(deadline) <= 0 {
			t.Error("missing/bad application deadline")
		}
		return nil, map[string]any{"bounded": ok}, nil
	})
	ss, err := server.Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client, err := mcp.NewClient(&mcp.Implementation{Name: "deadline-test"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "deadline_probe", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("deadline call: %v", err)
	}
}
