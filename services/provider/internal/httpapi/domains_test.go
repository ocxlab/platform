package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ocp.local/platform/internal/httpapi"
)

func TestDisabledDomainNeverDispatches(t *testing.T) {
	// A nil database makes unexpected authentication or business dispatch fail.
	handler, err := httpapi.NewHandler(nil, httpapi.WithDomains("none"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := httpapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	checked := map[string]bool{}
	for path, item := range spec.Paths.Map() {
		boundary, ok := item.Extensions["x-commerce-boundary"].(string)
		if !ok || (boundary != "shared" && boundary != "model" && boundary != "food") {
			t.Fatalf("unclassified path %s", path)
		}
		if boundary == "shared" {
			continue
		}
		for method, operation := range item.Operations() {
			actual := path
			for _, parameter := range []string{"merchantId", "storeId", "orderId", "quoteId", "mediaId", "uploadId"} {
				actual = strings.ReplaceAll(actual, "{"+parameter+"}", "11111111-1111-4111-8111-111111111111")
			}
			actual = strings.ReplaceAll(actual, "{orderReference}", "O-1")
			actual = strings.ReplaceAll(actual, "{storeHandle}", "demo-store")
			actual = strings.ReplaceAll(actual, "{revision}", "1")
			r := httptest.NewRequest(method, actual, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), `"code":"unavailable"`) {
				t.Fatalf("disabled %s %s: %d %s", method, path, w.Code, w.Body.String())
			}
			if w.Header().Get("X-Request-ID") == "" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing response controls")
			}
			count++
			checked[operation.OperationID] = true
		}
	}
	if count < 35 {
		t.Fatalf("domain annotations missing: only %d operations checked", count)
	}
	for _, operation := range []string{"GetPublicOffering", "WriteOffering", "ChangePublication", "WatchPublicOffering", "ListPublicStores", "GetPublicStoreByHandle", "GetPublicMediaImage", "CreateGuestSession", "AdminGetPublication"} {
		if !checked[operation] {
			t.Errorf("model-bound operation not checked: %s", operation)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/health/live", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("shared health disabled: %d", w.Code)
	}
}

func TestOnlyInstalledDomainsStart(t *testing.T) {
	for _, config := range []string{"shopping", "lodging", "food,food", "food,unknown"} {
		if _, err := httpapi.NewHandler(nil, httpapi.WithDomains(config)); err == nil {
			t.Fatalf("started uninstalled/invalid config %q", config)
		}
	}
	for _, config := range []string{"", "food"} {
		h, err := httpapi.NewHandler(nil, httpapi.WithDomains(config))
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/food/orders", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("enabled domain not dispatched to authorization: %d", w.Code)
		}
	}
}
