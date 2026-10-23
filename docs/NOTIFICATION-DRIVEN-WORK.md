# Notification-driven catalog work

23 October 2026. This implementation note explains how the catalog worker finds
committed work without its former one-second ticker. It is intended for maintainers
changing worker scheduling or extracting a runtime. PostgreSQL still owns durable
intent, retries, and leases; notifications only shorten the time until a worker
checks that state. The only supported effect is `food/catalog.publish`.

## Ownership and flow

Core records catalog delivery intent in the same transaction as publication.
Migration `00019_work_notifications.sql` adds a trigger that calls `pg_notify`
when an effect is inserted or its state, availability, or lease deadline changes.
PostgreSQL delivers the notification after commit. Its payload is only the domain
name, never a merchant, store, order, or catalog document.

The worker separates durable operations (`Store`) from disposable hints
(`WakeSource`). `SQLStore` implements claim, finish, and next-deadline lookup using
the existing persistence operations and generated sqlc query. Handlers receive
`workmodel.Effect`, not generated database rows. Enqueue remains part of the owning
business transaction, rather than becoming a separate publish call.

```text
Publication transaction ──> durable effect
          │                    │
     commit notification       │ claim / finish / next deadline
          ▼                    ▼
 PostgreSQL listener ──> worker scheduler ──> catalog delivery adapter
                      bounded Go channel      (outside SQL transaction)
```

`PostgresWake` establishes `LISTEN` before signaling the first scan, including
after reconnect. A one-slot Go channel coalesces hints. The worker scans current
state, drains at most eight batches, and asks the database for the earliest pending
retry or leased-work deadline. A relative delay calculated against database time
arms a local timer; another notification can interrupt that wait for earlier work.
The same schedule includes abandoned final-attempt leases so they become terminal
instead of remaining leased forever.

Stable effect IDs, bounded concurrency, retry limits, and generation/token fencing
are retained. A live lease prevents another normal claim, but cannot prove an
external request ran exactly once. Receivers still need idempotency or reconciliation
when an external result is ambiguous.

## Recovery and resource bounds

Missed notifications do not remove durable work. Startup and reconnect scans recover
the gap, deadline timers recover retries and expired leases, and a configurable
safety scan also works when the listener cannot connect at all. The default safety
interval is one minute; the worker-only `-work-recovery-interval` flag accepts
100 milliseconds through one hour. A shorter interval is a diagnostic/tuning option,
not a recommendation for idle production operation.

Drains start no more frequently than every 100 milliseconds. Overdue or contended
work uses that minimum delay rather than a tight loop. A storage error schedules
another attempt within five seconds, capped by the safety interval. Listener
connection attempts have a five-second timeout and reconnect backoff from one to
30 seconds. Repeated short-lived connections also increase that delay. A successful
listener attempt lasting at least 30 seconds resets it to one second; cancellation
interrupts the wait. Normal idle operation waits for a hint or safety timer; this is reduced
polling, not zero polling.

Each worker adds one dedicated PostgreSQL connection, identified as
`ocp-work-wake`; browser connections do not each acquire a database connection.
Use a direct or session-compatible PostgreSQL endpoint for `LISTEN`, not a
transaction-pooling endpoint. No broker or extra deployed runtime is introduced.
The existing browser SSE bridge and authorized state reads are unchanged.

Shutdown cancels the listener and handlers and stops further claims. Actual adapters
must honor cancellation: each handler gets a ten-second deadline, and finishing an
attempt has an independent three-second cleanup deadline. Listener cleanup waits at
most two seconds; the executable waits at most 15 seconds for worker shutdown.
Unfinished work retains a recoverable lease if cleanup cannot complete.

The next-deadline function uses the restricted work owner, a fixed search path,
validated domain inputs, and no PUBLIC execution grant. It exposes an aggregate
delay, not private effect records. Existing row security and claim/finish fencing
remain in force.

## Evidence and remaining work

Focused race-enabled tests exercised startup work, commit versus rollback, domain-only
notification payloads, schedule-function privileges, an earlier deadline interrupting
a wait, timed retries without another insert, abandoned lease expiry, exhausted
leases, two workers, and work inserted during forced listener reconnects. Unit tests
also cover coalescing, safety recovery without a hint, bounded backlog draining,
storage-error recovery, configuration validation, and cancellation-safe finishing.
The upgrade fixture starts at migration 18 with retained intent, applies migration
19, and delivers the existing intent afterward. These are local, same-author tests,
not independent interoperability or production capacity evidence.

The established `db:schema` / `db:draft` workflow was exercised against a disposable
runtime-schema database. Its pgschema draft omitted function ownership; the reviewed
migration explicitly assigns `ocp_work_owner` and the execution grants. Squawk and
historical-hash checks passed. Migration replay tests now choose their test migration
version from the existing history instead of reserving version 19.

Full regression and runtime results are recorded in [development status](STATUS.md).
Batch 3 adds optional backlog snapshots, worker telemetry, remaining recovery tests,
and bounded idle/burst observations; see [worker evidence](WORKER-EVIDENCE.md).
The [final engineering/protocol review](reviews/notification-worker-implementation.md)
accepts the bounded implementation. [Mutation testing](MUTATION-TESTING.md) records
the scheduler/attempt scope, surviving timing mutations, and coverage gaps.
This change does not implement production payment or kitchen work, alter Beckn
checkout, select hosting, or establish a cloud-cost saving.
