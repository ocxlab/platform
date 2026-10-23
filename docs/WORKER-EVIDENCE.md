# Catalog worker evidence

23 October 2026. This note records the local evidence for the notification-driven
worker and its instrumentation. It helps maintainers distinguish recovery guarantees
from performance observations. The scheduling model is explained in
[Notification-driven catalog work](NOTIFICATION-DRIVEN-WORK.md).

## Idle and burst observation

`task test:work-observations` runs a race-enabled Go test against disposable local
PostgreSQL databases. Both modes use the current durable Store/Once implementation,
concurrency two, and a no-op catalog delivery handler. The reference loop reproduces
the previous one-second ticker cadence; it is not an archived application binary.
After startup settles, the test observes about 3.2 idle seconds, commits twelve
separate intents, and waits until all twelve records are durably completed.

| Scheduler | Idle claim calls | Idle schedule calls | Twelve-item completion | Burst claim calls | Burst schedule calls |
| --- | ---: | ---: | ---: | ---: | ---: |
| One-second reference | 3 | 0 | 5,652.040 ms | 6 | 0 |
| Notification-driven | 0 | 0 | 23.033 ms | 7 | 1 |

These are two observations from one local run, not averages or capacity estimates.
The recorded calls are worker Store calls, not all PostgreSQL statements or I/O:
transaction setup, trigger execution, listening, and finishing work also consume
resources. Notification draining makes more calls during this burst, while avoiding
the reference loop's deliberate one-second waits. The burst timing includes enqueue
and completion but no external network delivery. The probe does not assert a
speedup; it checks cadence and bounded eventual completion. Raw values are retained in
[CSV](evidence/worker-scheduling-2026-10-23.csv).

The new worker's default safety scan is one minute, outside the idle observation
window. It therefore still polls infrequently. Each worker also holds one dedicated
listener connection. Neither the short window nor the no-op adapter establishes
cloud savings, production throughput, browser SSE cost, or real PSP/ONIX latency.
The schema in both modes includes notification triggers; this comparison isolates
scheduling rather than measuring the cost of adding those triggers.

## Recovery and isolation coverage

The PostgreSQL suite exercises commit versus rollback, startup work, reconnect gaps,
two workers, scheduled retries without new inserts, an earlier deadline, abandoned
leases, and final-attempt exhaustion. The existing fencing test rejects stale and
duplicate completion and checks tenant/domain isolation. New tests also recover
committed work using a silent wake source, persist a retry when a running handler
is canceled, reject unknown notification payloads, and verify snapshot-function
domain validation, restricted ownership, ACLs, age, and zero-valued domain rows.

Unit tests cover coalescing, the eight-batch drain limit, overdue timers, storage
and scheduling failures, wake-source exit, cancellation during reconnect backoff,
and optional snapshot failure. Snapshot failure does not stop durable delivery,
and repeated drains do not cause a snapshot query each time. Telemetry tests reject
private label values and exercise the bounded metric vocabulary. Existing SSE
authorization and reconnect tests remain part of regression; browser polling and
SSE behavior are unchanged.

Same-author receiver and native-network results and full regression outcomes are
recorded in [development status](STATUS.md). The final
[engineering/protocol review](reviews/notification-worker-implementation.md) accepts
the bounded implementation; [mutation testing](MUTATION-TESTING.md) records its
explicitly limited scope and remaining gaps. A test
receiver is not native ONIX, and neither is independent protocol conformance.

## Operational interpretation

Worker counters describe attempts, hints, and scheduling activity, not customer
orders or exactly-once external execution. Repeated claims include both ordinary
retries and expired-lease recovery; the current counter does not separate them.
Expired-lease gauges expose recovery pressure at snapshot time, not a count of
every recovered lease. Automatic exhaustion after an abandoned final lease appears
in the failed backlog; a handler failure on its final attempt is also classified
as `failed`, rather than misleadingly counted as a scheduled retry.

Migration 20 adds `created_at` and the restricted aggregate snapshot function.
New effects retain database enqueue time across retries. Legacy rows start their
measured age at the migration: the original enqueue time was not retained and is
not reconstructed. No prior migration is rewritten. The pgschema drafting workflow
was executed and its draft reviewed for schema qualification and omitted ownership;
Squawk passed the reviewed migration.

Snapshots are optional and enabled in the executable only with telemetry export.
They run on a drain at most once per minute, with a two-second timeout. Default idle
safety scans provide that opportunity; a longer configured safety interval can
make snapshots less frequent. A failure leaves the last gauge values stale and
increments `snapshot_failure`; do not treat missing or old telemetry as healthy
zero backlog. Aggregate scans are bounded in frequency and time, not in row count;
their cost and terminal-work retention need profiling as the retained table grows.

Backlog totals are duplicated across workers observing the same domain/database.
Each telemetry runtime supplies a random technical `service.instance.id` to keep
replica streams distinct; it is unrelated to commerce identities. Do not sum those
gauges across replicas. `delayed` is a subset of `pending`, and
`expired` a subset of `leased`; they are not disjoint totals. The oldest pending age
includes delayed pending retries. Operational telemetry is not an audit log or
merchant commercial analytics. No collector or dashboard deployment is required.
