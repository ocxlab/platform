package httpapi_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/offering"
	"ocp.local/platform/internal/offering/food"
)

func TestContractExamples(t *testing.T) {
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	if err = spec.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, example := range []struct{ file, schema string }{{"create-store.json", "CreateStoreRequest"}, {"food-menu.json", "FoodMenu"}, {"revision-conflict.json", "Problem"}} {
		t.Run(example.file, func(t *testing.T) {
			data, err := os.ReadFile("../../../../contracts/openapi/v1/examples/" + example.file)
			if err != nil {
				t.Fatal(err)
			}
			var value any
			if err = json.Unmarshal(data, &value); err != nil {
				t.Fatal(err)
			}
			if err = spec.Components.Schemas[example.schema].Value.VisitJSON(value); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestFoodDefinitions(t *testing.T) {
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	validator := food.Validator(spec.Components.Schemas["FoodMenu"].Value)
	raw, err := os.ReadFile("../../../../contracts/openapi/v1/examples/food-menu.json")
	if err != nil {
		t.Fatal(err)
	}
	registry := offering.NewRegistry(map[offering.Model]offering.Validator{food.Model: validator})
	if _, err = registry.Validate(offering.Model{ID: "uninstalled", Version: "1"}, raw); err != offering.ErrUnsupported {
		t.Fatal("unknown model accepted")
	}
	if _, err = registry.Validate(food.Model, raw); err != nil {
		t.Fatal(err)
	}
	for _, test := range []string{"duplicate_items", "fractional_price", "impossible_options", "unknown_field", "unavailable_item", "blank_group_name", "blank_option_name"} {
		t.Run(test, func(t *testing.T) {
			var doc map[string]any
			json.Unmarshal(raw, &doc)
			items := doc["items"].([]any)
			item := items[0].(map[string]any)
			switch test {
			case "duplicate_items":
				doc["items"] = append(items, item)
			case "fractional_price":
				item["variants"].([]any)[0].(map[string]any)["price_minor"] = 1.5
			case "impossible_options":
				item["option_groups"].([]any)[0].(map[string]any)["max_selected"] = 3
			case "unknown_field":
				doc["stock"] = 100
			case "unavailable_item":
				item["available"] = false
			case "blank_group_name":
				item["option_groups"].([]any)[0].(map[string]any)["name"] = "   "
			case "blank_option_name":
				item["option_groups"].([]any)[0].(map[string]any)["options"].([]any)[0].(map[string]any)["name"] = "   "
			}
			data, _ := json.Marshal(doc)
			_, err := validator(data)
			if test == "unavailable_item" {
				if err != nil {
					t.Fatal("manual unavailability rejected")
				}
			} else if err == nil {
				t.Fatal("invalid menu accepted")
			}
		})
	}
}

func TestProtectedOperationsDeclareScope(t *testing.T) {
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{"store:read": true, "store:write": true, "offering:read": true, "offering:write": true, "publication:read": true, "publication:manage": true, "quote:create": true, "order:create": true, "order:read": true, "order:manage": true, "order:cancel": true, "order:inspect": true}
	for _, path := range spec.Paths.Map() {
		for _, operation := range path.Operations() {
			access, ok := operation.Extensions["x-access"].(string)
			if !ok || (access != "merchant" && access != "provider" && access != "public" && access != "guest") {
				t.Fatalf("missing audience: %s", operation.OperationID)
			}
			for status, response := range operation.Responses.Map() {
				if strings.HasPrefix(status, "2") {
					header := response.Value.Headers["X-Request-ID"]
					if header == nil || header.Value == nil || header.Value.Schema.Value.Format != "uuid" {
						t.Fatalf("success response has no request identifier: %s %s", operation.OperationID, status)
					}
				}
			}
			if operation.OperationID == "GetLiveness" || operation.OperationID == "GetPublicOffering" || operation.OperationID == "CreateFoodQuote" || operation.OperationID == "CreateGuestSession" || operation.OperationID == "ListPublicStores" || operation.OperationID == "GetPublicStoreByHandle" {
				if access != "public" {
					t.Fatal("public operation audience mismatch")
				}
				if operation.Security == nil || len(*operation.Security) != 0 {
					t.Fatal("public operation requires explicit security override")
				}
				continue
			}
			scope, ok := operation.Extensions["x-required-scope"].(string)
			if !ok || !known[scope] {
				t.Fatalf("missing action policy: %s", operation.OperationID)
			}
		}
	}
}
