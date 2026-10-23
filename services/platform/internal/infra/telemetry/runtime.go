// Package telemetry owns operational signals, never commerce payloads or audit data.
package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

type Config struct {
	Exporter    string
	SampleRatio float64
}

// ConfigFromEnv deliberately does not enable outbound telemetry by default.
// OTLP endpoint, headers, and TLS settings use the SDK's standard environment.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	c := Config{Exporter: strings.TrimSpace(getenv("OCP_OTEL_EXPORTER")), SampleRatio: 0.1}
	if c.Exporter == "" {
		c.Exporter = "none"
	}
	if value := getenv("OCP_OTEL_TRACE_SAMPLE_RATIO"); value != "" {
		var err error
		c.SampleRatio, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return Config{}, errors.New("invalid telemetry sample ratio")
		}
	}
	return c, c.validate()
}

func (c Config) validate() error {
	if c.Exporter != "none" && c.Exporter != "otlp-http" {
		return errors.New("telemetry exporter must be none or otlp-http")
	}
	if math.IsNaN(c.SampleRatio) || math.IsInf(c.SampleRatio, 0) || c.SampleRatio < 0 || c.SampleRatio > 1 {
		return errors.New("telemetry sample ratio must be between zero and one")
	}
	return nil
}

type Runtime struct {
	Observer *Observer
	traces   *trace.TracerProvider
	metrics  *metric.MeterProvider
}

func runtimeResource() *resource.Resource {
	// Technical replica identity, never a merchant/customer identifier. Without
	// this, multiple workers would export indistinguishable backlog gauge streams.
	return resource.NewSchemaless(attribute.String("service.name", "ocp-provider"), attribute.String("service.instance.id", uuid.NewString()))
}

func New(ctx context.Context, c Config, logger *slog.Logger) (*Runtime, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	res := runtimeResource()
	traceOptions := []trace.TracerProviderOption{trace.WithResource(res), trace.WithSampler(trace.NeverSample())}
	metricOptions := []metric.Option{metric.WithResource(res), metric.WithCardinalityLimit(1000)}
	if c.Exporter == "otlp-http" {
		tx, err := otlptracehttp.New(ctx, otlptracehttp.WithTimeout(2*time.Second),
			otlptracehttp.WithRetry(otlptracehttp.RetryConfig{Enabled: false}))
		if err != nil {
			return nil, errors.New("cannot configure trace exporter")
		}
		mx, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithTimeout(2*time.Second),
			otlpmetrichttp.WithRetry(otlpmetrichttp.RetryConfig{Enabled: false}))
		if err != nil {
			_ = tx.Shutdown(ctx)
			return nil, errors.New("cannot configure metric exporter")
		}
		// Ratio sampling is independent of untrusted inbound sampling flags.
		traceOptions = append(traceOptions, trace.WithSampler(trace.TraceIDRatioBased(c.SampleRatio)),
			trace.WithBatcher(tx, trace.WithMaxQueueSize(512), trace.WithMaxExportBatchSize(128),
				trace.WithBatchTimeout(5*time.Second), trace.WithExportTimeout(2*time.Second)))
		metricOptions = append(metricOptions, metric.WithReader(metric.NewPeriodicReader(mx,
			metric.WithInterval(30*time.Second), metric.WithTimeout(2*time.Second))))
	}
	r := &Runtime{traces: trace.NewTracerProvider(traceOptions...), metrics: metric.NewMeterProvider(metricOptions...)}
	var err error
	r.Observer, err = NewObserver(r.traces, r.metrics, logger)
	if err != nil {
		_ = r.Shutdown(ctx)
		return nil, err
	}
	return r, nil
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	return errors.Join(r.traces.Shutdown(ctx), r.metrics.Shutdown(ctx))
}

// InstallErrorHandler keeps SDK exporter failures safe and rate-limited. Do not
// log arbitrary exporter errors: they can include endpoint URLs or configuration.
func InstallErrorHandler(logger *slog.Logger) {
	var last atomic.Int64
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) {
		now := time.Now().Unix()
		previous := last.Load()
		if now-previous >= 30 && last.CompareAndSwap(previous, now) {
			logger.Warn("telemetry export failed; commerce remains available")
		}
	}))
}
