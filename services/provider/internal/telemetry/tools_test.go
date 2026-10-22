package telemetry

import (
	"context"
	"encoding/json"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"strings"
	"testing"
)

func TestMerchantToolSignalsAreBounded(t *testing.T) {
	o, exporter, reader, logs := fixture(t)
	for _, tc := range []struct{ name, code string }{
		{"inspect_menu", ""}, {"inspect_menu", "revision_conflict"},
		{"set_item_availability", "unavailable"}, {"private-client-supplied-tool-name", "raw-private-error"},
		{"inspect_menu_item", ""}, {"inspect_item_extras", "not_found"},
	} {
		ctx, finish := o.StartMerchantTool(context.Background(), tc.name)
		if ctx == nil {
			t.Fatal("missing context")
		}
		finish(tc.code)
	}
	spans := exporter.GetSpans()
	if len(spans) != 6 {
		t.Fatal(len(spans))
	}
	if attrs(spans[1].Attributes)["ocp.outcome"] != "rejected" || attrs(spans[3].Attributes)["ocp.mcp.tool"] != "_OTHER" {
		t.Fatal("unbounded/wrong classification")
	}
	if attrs(spans[4].Attributes)["ocp.mcp.tool"] != "inspect_menu_item" || attrs(spans[5].Attributes)["ocp.mcp.tool"] != "inspect_item_extras" {
		t.Fatal("detail tools missing from bounded labels")
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
	if strings.Contains(string(wire), "private-client-supplied") || strings.Contains(string(wire), "raw-private-error") {
		t.Fatal("private data escaped into telemetry")
	}
	if !strings.Contains(string(wire), "ocp.mcp.tool.calls") || !strings.Contains(string(wire), "ocp.mcp.tool.duration") {
		t.Fatal("missing tool metrics")
	}
}
