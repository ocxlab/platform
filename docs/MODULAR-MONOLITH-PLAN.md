# Modular monolith and notification-driven work

23 October 2026. Status: batches 1–4 verified. This plan turns the engineering,
protocol, and Astra design reviews into bounded implementation batches. Review
record: sibling `ocp-lab/runs/notification-driven-work-review.md`.

## Objective and decisions

Keep the inexpensive combined runtime while making module ownership explicit and
allowing selected capabilities to move into independently scaled processes later.
This is a growth path, not a promise of zero extraction work or proven cost savings.
No additional deployed service is required by this plan. Listener connections and
timers consume resources; their benefit must be observed rather than assumed.

Use ordinary local calls for commands and immediate results. Use bounded Go
channels for disposable local wake-ups and existing SSE fan-out. Do not introduce
Watermill now: revisit it only if its routing/middleware meaningfully reduces
concrete duplication. Do not convert every interaction into an asynchronous event.

Required external work remains durably recorded alongside its owning business
decision. PostgreSQL notifications wake workers; they do not carry durable work or
prove completion. Keep the existing cross-process SSE bridge and authorized state
reads. Do not introduce browser polling.

Keep HTTP contracts in OpenAPI. Define reusable message payload schemas separately
when implementing message changes; defer AsyncAPI itself until channel/operation
documentation or generation has concrete value. This supersedes the research
evaluation's earlier recommendation to adopt a thin AsyncAPI document immediately.

## Batch 1: Ownership and extraction boundaries

- [x] Inventory query inputs, table readers/writers, application interfaces, and
  existing cross-owner transactions. Distinguish schema names from logical owners.
- [x] Record table/write ownership and explicit, narrowly scoped exceptions in a
  maintainable ownership manifest and a short explanation.
- [x] Add architecture tests rejecting newly introduced cross-owner SQL writes
  and forbidden implementation imports. Add negative fixtures proving the guards
  detect violations; document limitations rather than claiming a complete SQL parser.
- [x] Check existing payment calls and stable operation identities. Keep the
  development payment simulator's shared transaction as an explicit temporary
  exception, not the production payment integration template.
- [x] Audit small owned application interfaces at the boundaries touched by this
  work. Avoid transaction handles and persistence structs in extraction-facing
  interfaces. Do not rewrite all existing persistence code or add empty abstractions.
- [x] Record where immediate atomicity requires capabilities to stay together.

Inventory and guard limitations: [Module ownership](MODULE-OWNERSHIP.md). No runtime
boundary is changed in batch 1, so no empty interface is introduced. Worker store
and wake interfaces belong to batch 2, where their implementation will be exercised.

Exit: ownership is visible, existing exceptions are bounded, and tests prevent
new coupling from silently accumulating. Existing behavior and deployment stay
unchanged. Do not split databases or introduce remote calls.

## Batch 2: Notification-driven durable worker

- [x] Separate durable enqueue/claim/finish/scheduling operations from a small
  wake-source adapter; inject time/wake dependencies where useful for tests.
- [x] Add a new Goose migration for transactional wake signaling. Preserve the
  historical migrations; use the established schema/draft/lint workflow.
  Payloads contain only a minimal wake/domain hint, not private commerce records.
- [x] Add a dedicated PostgreSQL listener, bounded coalescing Go wake channel,
  reconnect handling, and cancellation. Establish LISTEN before scanning after
  startup and reconnection.
- [x] Replace the one-second idle ticker with bounded draining on startup/wake,
  earliest-due retry/lease timers, and an infrequent configurable safety scan.
  Query the schedule through sqlc, with domain-scoped privileged access as needed;
  ordinary callers must not gain cross-tenant data access.
- [x] Cover changes to the next due time, coalesced signals, peer claims, dead-peer
  lease expiry, exhausted attempts, listener failure, and an always-busy backlog.
  Avoid past-due timer spin and unbounded notification-triggered queries.
- [x] Preserve stable effect IDs, bounded concurrency, retry limits, lease fencing,
  and external calls outside SQL transactions. Stop claiming during shutdown and
  allow bounded cleanup; unfinished leases remain recoverable.

Exit: normal idle operation no longer issues a one-second claim query. Committed
work remains recoverable without receiving its original notification. Existing
`food/catalog.publish` is the only effect kind in this batch; payment/kitchen
workflows are not implemented by generalizing the worker.

Implementation and focused evidence: [Notification-driven catalog work](NOTIFICATION-DRIVEN-WORK.md).
The batch 2 tests already exercise several batch 3 recovery scenarios; that does
not complete its observability, measurement, and remaining failure-test work.

## Batch 3: Observability and failure tests

- [x] Extend existing OTel instrumentation for wake/reconnect events, empty drains,
  claim/handler outcomes, pending-work age, delayed retries, lease recovery, and
  terminal failures. Keep metric labels low-cardinality and free of tenant IDs,
  order IDs, secrets, or notification bodies. Do not add a monitoring deployment.
- [x] Test commit versus rollback, work before listener startup, reconnect gaps,
  missed/coalesced signals, and a backlog larger than one batch.
- [x] Test scheduled retries without another insert, dead-worker lease expiry,
  stale completion rejection, retry exhaustion, and bounded graceful shutdown.
- [x] Test two workers against the same database: one live claimant, safe recovery,
  and stable identities for repeat external attempts. Do not claim exactly-once
  external effects from exclusive claims.
- [x] Preserve tenant/domain isolation and test wake-payload privacy. Reuse current
  SSE authorization/reconnect tests; do not redesign SSE in this batch.
- [x] Exercise catalog delivery after worker outage and preserve generation ordering
  using the existing local receiver/network harness; distinguish synthetic receiver
  coverage from native ONIX or independent interoperability.
- [x] Observe idle query cadence and a bounded burst locally before/after. Record
  actual results and limitations, not production capacity or cloud-cost estimates.

Exit: failure/recovery behavior is evidenced and query/latency observations are
recorded. Timers and safety scans are explicit, not a claim of zero polling.

Results and metric interpretation: [Worker evidence](WORKER-EVIDENCE.md). The
reference cadence probe uses today's Store with the old ticker, not an archived
binary. Recovery tests include the existing batch 2 coverage rerun in regression.

## Batch 4: Regression, review, and handoff

- [x] Use Docker-backed Task commands, regenerate sqlc/OpenAPI artifacts as needed,
  and keep generated code ignored. Preserve development data.
- [x] Run focused tests, migration checks, `task check`, `task smoke`, and
  `task test:domains` for affected runtime/recovery paths. Run relevant existing
  network checks when publisher wiring changes. Use targeted Go mutation testing
  for new scheduling/guard logic where practical; report exclusions and survivors.
- [x] Obtain engineering and protocol reviews of the implemented change and its
  evidence; resolve material findings before calling the batch complete.
- [x] Update this checklist, `docs/STATUS.md`, and the research status/progress
  references. Inspect staged changes and commit coherent verified chunks locally.

Stopping point: the existing catalog worker runs notification-first with durable
recovery, ownership guards protect the relevant seams, SSE and product behavior
remain intact, and the implementation has passed review and regression.

Completed evidence: [implementation review](reviews/notification-worker-implementation.md),
[mutation scope and gaps](MUTATION-TESTING.md), and [development status](STATUS.md).
The protocol review's optional trigger-owner hardening remains documented follow-up,
not a blocking finding or an unimplemented part of the accepted scheduling behavior.

## Deferred growth path and subsequent product work

When measured demand justifies it, increase bounded worker concurrency or use the
already selectable processes first. Profile the actual limiting resource before
choosing a broker or separating storage. Preserve authentication, tenant scope,
and domain-specific lifecycle semantics when extracting a capability.

Redis Pub/Sub may later replace disposable cross-process hints. Redis Streams or
another durable transport needs explicit delivery/acknowledgment/retry semantics;
database intent plus a reliable outbox/CDC relay still addresses atomic publication.
Moving a local call across a network also needs timeouts, idempotent retry identities,
retained outcomes, and partial-failure handling. Keep decision and required intent
transactionally colocated with their owner; do not require every extracted service
to write one central work table. No Redis implementation or CDC infrastructure now.

Do not expand this batch into production payment orchestration, kitchen integration,
new Shopping/Lodging workflows, provider migration, cloud selection, schema-history
squashing, a UI redesign, or a generic workflow engine. After this cleanup, resume
the pending API/MCP work (including Core account management and intent-oriented
Food tools), then provider administration, followed by the planned Beckn/ONIX work.
Existing catalog/network behavior must remain working throughout.

Frontend state-management ideas involving XState/TanStack are deferred for a
separate user discussion; this worker plan does not adopt them or change the UI.
