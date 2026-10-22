# Operational telemetry

The Go provider now instruments its REST request boundary with OpenTelemetry.
It records request counts, duration histograms, and in-flight requests, plus
server spans and safe failure logs correlated by request, trace, and span IDs.
This is the foundation for the planned merchant/customer MCP endpoints and
provider-admin **MCPs** tab, not an implementation of those interfaces yet.

## Run locally

Normal `task dev` needs no monitoring service. Export defaults to `none`: no
outbound telemetry is sent, traces are not sampled, and there is no retained
metrics reader. Safe failure logs still appear through `task logs`. No statistics
dashboard or retained trace history is available in that mode.

To use an existing OTLP/HTTP receiver, configure the ignored root `.env`:

```dotenv
OCP_OTEL_EXPORTER=otlp-http
OCP_OTEL_TRACE_SAMPLE_RATIO=0.1
OTEL_EXPORTER_OTLP_ENDPOINT=http://host.docker.internal:4318
```

Then run `task dev` to apply the container configuration. The example hostname
reaches a receiver on the Docker Desktop host; choose the appropriate address
for another deployment. No receiver is installed or started by this repository.
Do not point telemetry at an unapproved external service. Use HTTPS outside a
trusted local environment. The runtime image includes system CA certificates.

The local Compose file also passes `OTEL_EXPORTER_OTLP_HEADERS` for a receiver
that requires authorization. Keep it in ignored configuration, not committed
files or shell output. The runtime supports the HTTP exporters' standard endpoint,
header, and TLS environment settings; other deployment wiring must explicitly
pass settings not exposed by the local Compose file. The exporter is HTTP/protobuf,
not gRPC. Its sample-ratio setting is our explicit application setting, not an
implementation of every OTel environment configuration option.

## Signals and safeguards

| Signal | Measurement | Bounded dimensions |
| --- | --- | --- |
| `http.server.request.duration` | Completed request duration, seconds, with histogram buckets | Method, route template, operation, status, outcome, and registered problem code when present |
| `http.server.active_requests` | Requests currently in progress | Method, route template, and operation |
| `ocp.http.server.requests` | Completed request attempts, including failures and replay attempts | Method, route template, operation, status, outcome, and registered problem code when present |

`ocp.*` names are application-specific; they are not represented as standard MCP
conventions. Unknown paths share `unmatched`, and unknown methods share `_OTHER`.
The metric provider caps cardinality at 1,000 data points per instrument per
collection cycle. Trace sampling defaults to 10% when export is enabled; metrics
are independent of that sampling. SDK sampling does not follow inbound sampling
flags. W3C trace IDs propagate for correlation, but neither incoming baggage nor
tracestate is collected. Trace context does not authorize a request.

Outcomes distinguish success, rejected requests, unauthenticated requests,
forbidden requests, rate limits, cancellation, timeout, and internal errors.
Expected 4xx responses do not mark a server span as an internal failure. These
HTTP measurements do not yet classify MCP tool errors inside successful HTTP
responses; that requires instrumentation at the future tool execution boundary.

Only allowlisted route/operation metadata and registered problem codes are
recorded. No request/response bodies, raw URLs or query strings, bearer tokens,
cookies, user-agent strings, merchant/customer IDs, SQL values, or error details
are captured. Failure logs contain safe metadata, not raw exception text. Health
checks are measured but do not produce per-failure access logs.

Trace exports use a non-blocking queue capped at 512 spans and batches of 128.
Exports time out after two seconds without automatic retry, and metrics export
every 30 seconds. Shutdown allows three seconds for draining. Export failures
do not change API responses; SDK export-error logs are safe and limited to one
per 30 seconds. Request failure logs remain normal operational logs and need
deployment-level log retention/rate controls before public exposure.

## Scope and remaining implementation

Current instrumentation covers the HTTP boundary and its existing authorization
and handler execution as one server span. Individual database queries, application
operations, payment calls, and MCP tools do not yet have child spans. We have not
changed token validation or implemented OAuth by adding telemetry.

No collector, Grafana LGTM stack, telemetry database, or administration endpoint
was added. The [management plan](OBSERVABILITY-AND-MCP-OPERATIONS.md) defines the
future MCPs tab and bounded retained summaries. Operational telemetry is not the
business/security audit or merchant commercial analytics.

The OTel API, SDK, and HTTP exporters are pinned to `v1.47.0` in `go.mod` and
`go.sum`. Their downloaded module LICENSE files identify Apache License 2.0.
Go module version selection also updates shared dependencies, including the
OpenAPI runtime required by the OTLP gateway dependency; contract drift and
integration checks cover that change. This is not a complete dependency-license
or production-security audit.

Run `task test:telemetry` for focused race-enabled tests. They exercise route and
error classification, cancellation, safe attributes/logs, response-controller
streaming, panic propagation, disabled export, a local OTLP/HTTP receiver, and
receiver failure. These are local tests, not a deployed-monitoring or load test.
