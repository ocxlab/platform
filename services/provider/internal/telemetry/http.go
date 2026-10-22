package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"ocp.local/platform/internal/problems"
)

type Observer struct {
	tracer   trace.Tracer
	duration metric.Float64Histogram
	requests metric.Int64Counter
	active   metric.Int64UpDownCounter
	logger   *slog.Logger
}

func NewObserver(tp trace.TracerProvider, mp metric.MeterProvider, logger *slog.Logger) (*Observer, error) {
	meter := mp.Meter("ocp.local/platform/provider")
	duration, err := meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 120))
	if err != nil {
		return nil, err
	}
	requests, err := meter.Int64Counter("ocp.http.server.requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, err
	}
	active, err := meter.Int64UpDownCounter("http.server.active_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, err
	}
	return &Observer{tp.Tracer("ocp.local/platform/provider"), duration, requests, active, logger}, nil
}

// HTTP receives only a route template and operation name from our contract
// router. It never derives labels from paths, headers, principals, or payloads.
// The callback retains the same response writer semantics, including SSE.
func (o *Observer) HTTP(w http.ResponseWriter, r *http.Request, route, operation string, next http.HandlerFunc) {
	method := knownMethod(r.Method)
	attrs := []attribute.KeyValue{attribute.String("http.request.method", method),
		attribute.String("http.route", route), attribute.String("ocp.operation", operation)}
	// Only W3C trace context, never baggage. A parent trace is not an identity.
	parent := propagation.MapCarrier{"traceparent": r.Header.Get("traceparent")}
	ctx := propagation.TraceContext{}.Extract(r.Context(), parent)
	ctx, span := o.tracer.Start(ctx, method+" "+route, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(attrs...))
	r = r.WithContext(ctx)
	recorder := &response{ResponseWriter: w}
	start := time.Now()
	o.active.Add(ctx, 1, metric.WithAttributes(attrs...))
	defer func() {
		panicValue := recover()
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		outcome := classify(status, r.Context().Err())
		if panicValue != nil {
			status, outcome = http.StatusInternalServerError, "internal_error"
		}
		finished := append(append([]attribute.KeyValue{}, attrs...), attribute.Int("http.response.status_code", status), attribute.String("ocp.outcome", outcome))
		if recorder.code != "" {
			finished = append(finished, attribute.String("ocp.error.code", recorder.code))
		}
		span.SetAttributes(finished...)
		if status >= 500 || outcome == "timeout" || outcome == "internal_error" {
			span.SetStatus(codes.Error, outcome)
		}
		o.active.Add(ctx, -1, metric.WithAttributes(attrs...))
		o.requests.Add(ctx, 1, metric.WithAttributes(finished...))
		o.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(finished...))
		if outcome != "success" && operation != "GetLiveness" && o.logger != nil {
			fields := []any{"operation", operation, "outcome", outcome, "status", status,
				"code", recorder.code, "request_id", w.Header().Get("X-Request-ID"),
				"trace_id", span.SpanContext().TraceID().String(), "span_id", span.SpanContext().SpanID().String()}
			o.logger.InfoContext(ctx, "API request completed", fields...)
		}
		span.End()
		if panicValue != nil {
			panic(panicValue)
		}
	}()
	next(recorder, r)
}

func knownMethod(method string) string {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE":
		return method
	default:
		return "_OTHER"
	}
}

func classify(status int, err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	switch {
	case status >= 500:
		return "internal_error"
	case status == 401:
		return "unauthenticated"
	case status == 403:
		return "forbidden"
	case status == 429:
		return "rate_limited"
	case status >= 400:
		return "rejected"
	default:
		return "success"
	}
}

type response struct {
	http.ResponseWriter
	status int
	code   string
}

func (w *response) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *response) WriteHeader(status int) {
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *response) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// ResponseController uses FlushError and Unwrap, preserving deadlines/streaming.
func (w *response) FlushError() error {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *response) SetProblemCode(code string) {
	if _, ok := problems.Lookup(code); ok {
		w.code = code
	}
}
