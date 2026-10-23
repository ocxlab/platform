package telemetry

import (
	"context"
	"encoding/json"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"ocp.local/platform/internal/core/capability"
	"strings"
	"testing"
)

func TestEffectSignalsKeepBoundedLabels(t *testing.T) {
	o, exporter, reader, logs := fixture(t)
	_, done := o.StartEffect(context.Background(), capability.Food, "catalog.publish")
	done("completed")
	_, done = o.StartEffect(context.Background(), "private-merchant", "private-payload")
	done("private-result")
	spans := exporter.GetSpans()
	if len(spans) != 2 || attrs(spans[1].Attributes)["ocp.work.kind"] != "_OTHER" {
		t.Fatal("unbounded effect label")
	}
	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(struct {
		Spans   any
		Metrics any
		Logs    string
	}{spans, data, logs.String()})
	if strings.Contains(string(wire), "private-") {
		t.Fatal("private effect data escaped")
	}
	if !strings.Contains(string(wire), "ocp.work.attempts") {
		t.Fatal("missing work metric")
	}
}
