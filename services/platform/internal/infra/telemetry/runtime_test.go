package telemetry

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
)

func TestConfigValidatesSafeDefaultsAndInvalidValues(t *testing.T) {
	config, err := ConfigFromEnv(func(string) string { return "" })
	if err != nil || config.Exporter != "none" || config.SampleRatio != .1 {
		t.Fatal("unsafe defaults")
	}
	for _, values := range []map[string]string{
		{"OCP_OTEL_EXPORTER": "grpc"},
		{"OCP_OTEL_TRACE_SAMPLE_RATIO": "-1"},
		{"OCP_OTEL_TRACE_SAMPLE_RATIO": "1.1"},
		{"OCP_OTEL_TRACE_SAMPLE_RATIO": "NaN"},
		{"OCP_OTEL_TRACE_SAMPLE_RATIO": "+Inf"},
		{"OCP_OTEL_TRACE_SAMPLE_RATIO": "private-invalid-value"},
	} {
		_, err := ConfigFromEnv(func(key string) string { return values[key] })
		if err == nil || strings.Contains(err.Error(), "private-invalid-value") {
			t.Fatal("invalid configuration accepted or leaked")
		}
	}
}

func TestDisabledExporterSendsNothing(t *testing.T) {
	var calls atomic.Int64
	receiver := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer receiver.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", receiver.URL)
	runtime, err := New(context.Background(), Config{Exporter: "none", SampleRatio: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runtime.Observer.HTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil), "/", "GetStore", func(http.ResponseWriter, *http.Request) {})
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("disabled exporter sent telemetry")
	}
}

func TestOTLPHTTPExportsSignalsWithoutPayloads(t *testing.T) {
	var traces, metrics atomic.Int64
	var leaked atomic.Bool
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "private-") {
			leaked.Store(true)
		}
		switch r.URL.Path {
		case "/v1/traces":
			traces.Add(1)
		case "/v1/metrics":
			metrics.Add(1)
		default:
			t.Errorf("unexpected export path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(200) // Empty protobuf ExportResponse is valid.
	}))
	defer receiver.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", receiver.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", receiver.URL+"/v1/traces")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", receiver.URL+"/v1/metrics")
	runtime, err := New(context.Background(), Config{Exporter: "otlp-http", SampleRatio: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	r := httptest.NewRequest("GET", "/private-store?secret=private-query", strings.NewReader("private-body"))
	r.Header.Set("Authorization", "Bearer private-token")
	runtime.Observer.HTTP(httptest.NewRecorder(), r, "/stores/{store}", "GetStore", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("private-response")) })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := runtime.traces.ForceFlush(ctx); err != nil {
		t.Fatal(err)
	}
	if err := runtime.metrics.ForceFlush(ctx); err != nil {
		t.Fatal(err)
	}
	if traces.Load() == 0 || metrics.Load() == 0 || leaked.Load() {
		t.Fatal("missing signals or private content exported")
	}
}

func TestUnavailableReceiverDoesNotFailRequests(t *testing.T) {
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
	defer receiver.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", receiver.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", receiver.URL+"/v1/traces")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", receiver.URL+"/v1/metrics")
	runtime, err := New(context.Background(), Config{Exporter: "otlp-http", SampleRatio: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	w := httptest.NewRecorder()
	runtime.Observer.HTTP(w, httptest.NewRequest("POST", "/orders", nil), "/orders", "PlaceFoodOrder", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(201) })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if runtime.traces.ForceFlush(ctx) == nil {
		t.Fatal("expected receiver error")
	}
	if runtime.metrics.ForceFlush(ctx) == nil {
		t.Fatal("expected receiver error")
	}
	if w.Code != 201 {
		t.Fatal("telemetry failure changed business response")
	}
}

func TestExporterErrorsAreSafeAndRateLimited(t *testing.T) {
	_, _, _, logs := fixture(t)
	previous := otel.GetErrorHandler()
	t.Cleanup(func() { otel.SetErrorHandler(previous) })
	InstallErrorHandler(slog.New(slog.NewJSONHandler(logs, nil)))
	for range 10 {
		otel.Handle(sensitiveError{})
	}
	if strings.Count(logs.String(), "telemetry export failed") != 1 || strings.Contains(logs.String(), "private-") {
		t.Fatal("unsafe or unbounded SDK logs")
	}
}

type sensitiveError struct{}

func (sensitiveError) Error() string { return "private-endpoint-with-private-credentials" }
