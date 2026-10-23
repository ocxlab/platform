package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"ocp.local/platform/internal/capability"
)

func TestHTTPDomainLabelsAreBounded(t *testing.T) {
	o, exporter, reader, _ := fixture(t)
	for _, domain := range []capability.Domain{capability.Food, capability.Shopping, capability.Lodging, "merchant-secret-not-a-domain"} {
		o.HTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/private", nil), "/route", "Read", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }, domain)
	}
	spans := exporter.GetSpans()
	if len(spans) != 4 {
		t.Fatalf("spans: %d", len(spans))
	}
	for i, want := range []string{"food", "shopping", "lodging", "shared"} {
		found := false
		for _, attr := range spans[i].Attributes {
			if string(attr.Key) == "ocp.domain" {
				found = true
				if attr.Value.AsString() != want {
					t.Fatalf("domain = %s, want %s", attr.Value.AsString(), want)
				}
			}
		}
		if !found {
			t.Fatal("missing domain label")
		}
	}
	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "merchant-secret-not-a-domain") {
		t.Fatal("unbounded domain leaked into metrics")
	}
}

func fixture(t *testing.T) (*Observer, *tracetest.InMemoryExporter, *sdkmetric.ManualReader, *bytes.Buffer) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	var logs bytes.Buffer
	o, err := NewObserver(tp, mp, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()); _ = mp.Shutdown(context.Background()) })
	return o, exporter, reader, &logs
}

func TestHTTPClassifiesOutcomesAndPreservesResponses(t *testing.T) {
	for _, tc := range []struct {
		status    int
		outcome   string
		spanError bool
	}{
		{200, "success", false}, {201, "success", false}, {401, "unauthenticated", false},
		{403, "forbidden", false}, {409, "rejected", false}, {429, "rate_limited", false},
		{503, "internal_error", true},
	} {
		t.Run(tc.outcome+http.StatusText(tc.status), func(t *testing.T) {
			o, exporter, reader, _ := fixture(t)
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/private-store?email=private@example.org", strings.NewReader("private order"))
			o.HTTP(w, r, "/stores/{store}/orders", "PlaceFoodOrder", func(w http.ResponseWriter, r *http.Request) {
				if !trace.SpanFromContext(r.Context()).SpanContext().IsValid() {
					t.Fatal("missing request span")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("unchanged response"))
			})
			if w.Code != tc.status || w.Body.String() != "unchanged response" {
				t.Fatal("changed API response")
			}
			spans := exporter.GetSpans()
			if len(spans) != 1 || spans[0].Name != "POST /stores/{store}/orders" {
				t.Fatalf("unexpected spans: %+v", spans)
			}
			if (spans[0].Status.Code == codes.Error) != tc.spanError {
				t.Fatal("wrong span status")
			}
			if attrs(spans[0].Attributes)["ocp.outcome"] != tc.outcome {
				t.Fatal("wrong outcome")
			}
			var data metricdata.ResourceMetrics
			if err := reader.Collect(context.Background(), &data); err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for _, scope := range data.ScopeMetrics {
				for _, m := range scope.Metrics {
					seen[m.Name] = true
					switch points := m.Data.(type) {
					case metricdata.Sum[int64]:
						if len(points.DataPoints) != 1 {
							t.Fatal("unexpected metric cardinality")
						}
						point := points.DataPoints[0]
						want := int64(1)
						if m.Name == "http.server.active_requests" {
							want = 0
						}
						if point.Value != want {
							t.Fatalf("%s: %d", m.Name, point.Value)
						}
					case metricdata.Histogram[float64]:
						if len(points.DataPoints) != 1 || points.DataPoints[0].Count != 1 {
							t.Fatal("missing duration")
						}
					}
				}
			}
			if len(seen) != 3 {
				t.Fatalf("missing instruments: %v", seen)
			}
		})
	}
}

func attrs(values []attribute.KeyValue) map[string]string {
	out := map[string]string{}
	for _, value := range values {
		out[string(value.Key)] = value.Value.String()
	}
	return out
}

func TestHTTPDoesNotCollectSensitiveContent(t *testing.T) {
	o, exporter, reader, logs := fixture(t)
	r := httptest.NewRequest("POST", "http://private-host/stores/private-store?email=private@example.org", strings.NewReader("private-body"))
	r.Header.Set("Authorization", "Bearer private-credential")
	r.Header.Set("Cookie", "session=private-cookie")
	r.Header.Set("Baggage", "secret=private-baggage")
	r.Header.Set("Tracestate", "vendor=private-tracestate")
	r.Header.Set("User-Agent", "private-client")
	r.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	w := httptest.NewRecorder()
	w.Header().Set("X-Request-ID", "server-generated-id")
	o.HTTP(w, r, "/stores/{store}", "GetStore", func(w http.ResponseWriter, r *http.Request) {
		w.(interface{ SetProblemCode(string) }).SetProblemCode("unavailable")
		w.WriteHeader(503)
		_, _ = w.Write([]byte("private-response"))
	})
	spans := exporter.GetSpans()
	if spans[0].SpanContext.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatal("lost trace context")
	}
	if spans[0].SpanContext.TraceState().String() != "" {
		t.Fatal("collected inbound tracestate")
	}
	if attrs(spans[0].Attributes)["ocp.error.code"] != "unavailable" {
		t.Fatal("missing safe code")
	}
	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(struct {
		Spans   any
		Metrics any
	}{spans, data})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{string(encoded), logs.String()} {
		if strings.Contains(text, "private-") || strings.Contains(text, "private@example.org") {
			t.Fatal("telemetry leaked sensitive data")
		}
	}
	if !strings.Contains(logs.String(), "server-generated-id") || !strings.Contains(logs.String(), "trace_id") {
		t.Fatal("missing correlation")
	}
}

func TestHTTPUnknownMethodsAndProblemCodesAreBounded(t *testing.T) {
	o, exporter, _, _ := fixture(t)
	r := httptest.NewRequest("CUSTOM_PRIVATE_METHOD", "/private", nil)
	o.HTTP(httptest.NewRecorder(), r, "unmatched", "unmatched", func(w http.ResponseWriter, _ *http.Request) {
		w.(interface{ SetProblemCode(string) }).SetProblemCode("private-unregistered-code")
		w.WriteHeader(404)
	})
	a := attrs(exporter.GetSpans()[0].Attributes)
	if a["http.request.method"] != "_OTHER" || a["ocp.error.code"] != "" {
		t.Fatal("unbounded metadata recorded")
	}
}

func TestHTTPRecordsCancellationAndDeadline(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		o, exporter, _, _ := fixture(t)
		var ctx context.Context
		var cancel context.CancelFunc
		want := "cancelled"
		if deadline {
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			want = "timeout"
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}
		cancel()
		o.HTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil).WithContext(ctx), "/", "GetStore", func(http.ResponseWriter, *http.Request) {})
		if attrs(exporter.GetSpans()[0].Attributes)["ocp.outcome"] != want {
			t.Fatal("wrong cancellation outcome")
		}
	}
}

func TestHTTPDoesNotSwallowPanics(t *testing.T) {
	o, exporter, _, logs := fixture(t)
	defer func() {
		if recover() != "private-panic" {
			t.Fatal("panic was swallowed or changed")
		}
		if attrs(exporter.GetSpans()[0].Attributes)["ocp.outcome"] != "internal_error" {
			t.Fatal("panic not recorded")
		}
		if strings.Contains(logs.String(), "private-panic") {
			t.Fatal("panic content leaked")
		}
	}()
	o.HTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil), "/", "GetStore", func(http.ResponseWriter, *http.Request) { panic("private-panic") })
}

func TestHTTPPreservesResponseControllerAndInformationalStatus(t *testing.T) {
	o, exporter, _, _ := fixture(t)
	w := &streamWriter{header: http.Header{}}
	o.HTTP(w, httptest.NewRequest("GET", "/stream", nil), "/stream", "WatchGuestFoodOrder", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(103)
		if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(500) // Must not overwrite the already flushed success status.
	})
	if !w.flushed || !w.deadline || len(w.statuses) != 2 || w.statuses[0] != 103 || w.statuses[1] != 200 {
		t.Fatal("stream behavior changed")
	}
	if attrs(exporter.GetSpans()[0].Attributes)["http.response.status_code"] != "200" {
		t.Fatal("wrong final status")
	}
}

type streamWriter struct {
	header            http.Header
	statuses          []int
	flushed, deadline bool
}

func (w *streamWriter) Header() http.Header              { return w.header }
func (w *streamWriter) WriteHeader(s int)                { w.statuses = append(w.statuses, s) }
func (w *streamWriter) Write(b []byte) (int, error)      { return len(b), nil }
func (w *streamWriter) FlushError() error                { w.flushed = true; return nil }
func (w *streamWriter) SetWriteDeadline(time.Time) error { w.deadline = true; return nil }

func TestClassifyWrappedContextErrors(t *testing.T) {
	if classify(200, errors.Join(errors.New("private"), context.Canceled)) != "cancelled" {
		t.Fatal("wrapped cancellation not classified")
	}
}
