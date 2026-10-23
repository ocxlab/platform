package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"ocp.local/platform/internal/httpapi"
)

func TestServiceRouteOwnershipBeforeAuthorizationOrStorage(t *testing.T) {
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	placeholder := regexp.MustCompile(`\{[^}]+\}`)
	for _, service := range []string{"core", "food"} {
		t.Run(service, func(t *testing.T) {
			// Wrong-service requests must not authenticate or touch this nil DB.
			h, err := httpapi.NewHandler(nil, httpapi.WithService(service))
			if err != nil {
				t.Fatal(err)
			}
			checked := 0
			for path, item := range spec.Paths.Map() {
				boundary := item.Extensions["x-commerce-boundary"].(string)
				if (service == "food") == (boundary == "food") {
					continue
				}
				for method, op := range item.Operations() {
					if op.OperationID == "GetLiveness" {
						continue
					}
					actual := strings.ReplaceAll(path, "{revision}", "1")
					actual = placeholder.ReplaceAllString(actual, "11111111-1111-4111-8111-111111111111")
					w := httptest.NewRecorder()
					h.ServeHTTP(w, httptest.NewRequest(method, actual, nil))
					if w.Code != 503 || !strings.Contains(w.Body.String(), `"code":"unavailable"`) {
						t.Fatalf("%s dispatches foreign %s %s: %d %s", service, method, path, w.Code, w.Body.String())
					}
					checked++
				}
			}
			if checked < 10 {
				t.Fatalf("too few opposite-service operations: %d", checked)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health/live", nil))
			if w.Code != 200 {
				t.Fatalf("service health: %d", w.Code)
			}
		})
	}
}

func TestUnknownPublicServiceDoesNotStart(t *testing.T) {
	if _, err := httpapi.NewHandler(nil, httpapi.WithService("shopping")); err == nil {
		t.Fatal("internal service admitted by public transport")
	}
}
