# Observability and MCP activity

Decision checkpoint: 22 October 2026, Europe/Berlin (UTC+02:00).
Status: implementation plan, not delivered instrumentation or a working dashboard.

## Purpose and ownership

Provider administrators need to understand whether customers and merchants can
use our MCP tools reliably, which operations fail, and where time is spent.
Add an **MCP activity** tab alongside Stores and Orders when the MCP surfaces
are implemented. “Activity” describes the two endpoints more accurately than
“fleet”: we are operating interfaces, not managing a fleet of autonomous agents.

The merchant and customer endpoints retain their four logical intent groups
from [the authorization plan](MCP-SURFACES-AND-AUTHORIZATION.md). Monitoring
does not merge their permissions or expose private tool content to administrators.

## What the tab should show

- Call counts, call rate, error rate, latency distribution, and in-flight calls,
  split by endpoint and named tool, with a selectable time window.
- Separate technical failures, expected business rejections, authentication or
  authorization denials, rate limits, cancellations, and timeouts. An HTTP success
  is not sufficient to classify a tool call as successful; inspect JSON-RPC
  errors and MCP tool error results as well.
- A paginated recent-failures table with timestamp, endpoint, tool, stable safe
  error code, duration, and request/trace identifiers for investigation.
- Instrumentation coverage, retention window, and dropped-event indications.
  Distinguish unavailable measurements from a measured zero.

Count request attempts separately from completed business operations: a replay
can be a successful tool response without creating another order. Tool volume
is not a count of distinct users or agents. Unverified client metadata is not
an identity. MCP call counts are also not model-token usage; model usage belongs
to the later chat instrumentation when actual usage information is available.

Use the separately authorized provider-administration API, with a dedicated
read permission, rather than direct database access from the UI. Follow existing
pagination, localization (English, German, and Arabic), and RTL conventions.

## OpenTelemetry without a required monitoring stack

Start OTel traces and metrics with the MCP implementation and instrument the
shared backend paths used by REST as well. Trace inbound requests, authorization,
tool execution, shared application operations, database access, and outbound
payment calls. Keep existing structured `slog` logging and attach request/trace
identifiers; a separate OTel log bridge is not a prerequisite.

Use published HTTP/RPC semantic conventions where applicable. Pin SDK and
convention versions during implementation; check MCP-specific convention maturity
before adopting it. Do not invent a supposedly standardized MCP metric vocabulary.
Use route templates and registered tool/error names as bounded metric dimensions,
not raw URLs, order IDs, account IDs, or arbitrary client-supplied strings.

Make OTLP export configurable, with bounded asynchronous buffering and timeouts.
An unavailable exporter must not fail an order. No mandatory Collector, Grafana,
Loki, Tempo, or Mimir deployment is required for the first implementation.

OTel instrumentation is not a statistics database. Supply the admin tab with
bounded operational rollups and a capped recent-failure store in the existing
PostgreSQL deployment, through a lightweight asynchronous path. Set retention
and storage limits before enabling it. Do not store every trace or synchronously
insert an analytics row for every request. Aggregation must preserve the latency
distribution rather than average per-process percentiles.

Use unsampled counters/histograms for usage and latency; sampled traces support
diagnosis, not exact totals. Buffered operational statistics can lose observations
on a crash and are not billing records or the durable business/security audit.
Make that distinction visible rather than promising exact accounting.

## Privacy and safety

Do not collect raw tool inputs/results, prompts, menu imports, order contents,
customer contact details, credentials, authorization headers, or SQL parameter
values. Use allowlisted metadata and safe error codes, not arbitrary exception
messages. Trace context is correlation data, never authorization; do not propagate
private data in baggage. Bound and validate incoming correlation metadata.

Telemetry is for service operation, not cross-merchant commercial analytics.
Opt-in merchant analytics remains a separate permission and product decision.

## Implementation order and checks

1. Establish the shared API/MCP authentication and authorization foundation.
   Existing opaque bearer grants are not OAuth. Reuse ownership/scope policy;
   support resource-appropriate tokens rather than unrestricted token passthrough.
2. Add backend OTel instrumentation alongside the first intent-based MCP tools.
   Cover success, tool errors, denied access, retries, and cancellation in tests
   using an in-memory exporter. Verify that sensitive values are absent.
3. Add the admin API and MCP activity tab with bounded storage. Exercise exporter
   failure, telemetry backpressure, pagination, retention, and both MCP endpoints.
4. Add chat/model instrumentation when chat exists; connect an OTLP-compatible
   monitoring backend later without changing the business interfaces.

## Primary documentation consulted

Retrieved 22 October 2026, Europe/Berlin. These are external implementation
references, not evidence that our code already implements the plan.

- [OTel Go](https://opentelemetry.io/docs/languages/go/), “Status and Releases”:
  traces and metrics are stable; logs are release candidate at this retrieval.
- [Go exporters](https://opentelemetry.io/docs/languages/go/exporters/), “OTLP”:
  export to an OTLP endpoint; no particular observability vendor is required.
- [Handling sensitive data](https://opentelemetry.io/docs/security/handling-sensitive-data/),
  “Data minimization”: collect only necessary information; responsibility for
  sensitive-data handling remains with the implementer.
