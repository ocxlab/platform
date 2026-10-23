package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"ocp.local/platform/internal/agenttools"
	"ocp.local/platform/internal/infra/telemetry"
)

func TestMerchantMCPUsesFoodDomainTelemetry(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	mp := sdkmetric.NewMeterProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()); _ = mp.Shutdown(context.Background()) })
	o, err := telemetry.NewObserver(tp, mp, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(nil, WithTelemetry(o), WithLocalMerchantMCP())
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", agenttools.MerchantPath, nil))
	for _, span := range exporter.GetSpans() {
		for _, a := range span.Attributes {
			if a.Key == attribute.Key("ocp.domain") && a.Value.AsString() == "food" {
				return
			}
		}
	}
	t.Fatal("Food MCP request did not carry Food domain telemetry")
}

func TestContractRoutingSuppliesSafeTelemetryAndProblemCodes(t *testing.T) {
	for _, tc := range []struct{ path, method, code string }{
		{"/private-unknown-path?email=private@example.org", "GET", "not_found"},
		{"/api/v1/merchants/11111111-1111-4111-8111-111111111111/stores", "GET", "unauthenticated"},
		{"/health/live", "GET", ""},
	} {
		exporter := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
		mp := sdkmetric.NewMeterProvider()
		o, err := telemetry.NewObserver(tp, mp, nil)
		if err != nil {
			t.Fatal(err)
		}
		handler, err := NewHandler(nil, WithTelemetry(o))
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		spans := exporter.GetSpans()
		if len(spans) != 1 {
			t.Fatal("missing request span")
		}
		var code string
		for _, a := range spans[0].Attributes {
			if a.Key == attribute.Key("ocp.error.code") {
				code = a.Value.AsString()
			}
			if strings.Contains(a.Value.String(), "private-") || strings.Contains(a.Value.String(), "11111111") {
				t.Fatal("raw path leaked")
			}
		}
		if code != tc.code {
			t.Fatalf("wrong problem code: %s", code)
		}
		if tc.code == "unauthenticated" && w.Code != http.StatusUnauthorized {
			t.Fatal("authentication changed")
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Fatal("missing request correlation")
		}
		_ = tp.Shutdown(context.Background())
		_ = mp.Shutdown(context.Background())
	}
}
