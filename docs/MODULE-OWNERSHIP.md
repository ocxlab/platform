# Module ownership and extraction boundaries

23 October 2026. This inventory describes the current implementation, not seven
independent databases. Keeping modules in one process is inexpensive; extracting
one later requires preserving the decisions currently protected by shared SQL
transactions. The inventory accompanies the ownership guards and the subsequent
notification-driven worker change.

## Data ownership

The executable inventory is
[`ownership.json`](../services/platform/tests/architecture/ownership.json).
It declares logical owners for 32 tables and 17 functions and eleven query-specific
foreign-access exceptions across the 105 named SQL inputs. PostgreSQL schema names are historical
storage names, not service boundaries.

| Owner | Owned records | Current application seam |
| --- | --- | --- |
| Core | Merchants, grants, stores, offering revisions, publication audit, media, replays, catalog delivery intents | Authorized Offering/Publication/Media operations; shared credential and transaction helpers |
| Food | Quotes, orders, operational state, order events, ordering audit | Food persistence operations and merchant MCP adapter; still coupled to Core transaction/principal types |
| Shopping | Catalogs, stock, sessions, orders | Internal domain implementation of `sessionkit.Domain` |
| Lodging | Properties, units, sessions, bookings, allocations | Internal domain implementation of `sessionkit.Domain` |
| Payment | Development financial operations and pay-later obligations | Validated `Terms` and `Subject`; local `RecordPayLater` transaction participant |
| Discovery | Enrolled catalog bindings and public catalog projections | Signed catalog delivery and enrolled projection acceptance |
| Infra | Technical adapters and migration execution, no business tables | Database setup, HTTP runtime, media storage, telemetry |

Core owns the internal Shopping/Lodging credential and replay tables even though
their current query inputs live in the internal HTTP transport. The `runtime.effects`
table is currently Core-owned publication coordination, not a license for every
future domain to write a universal shared work table. All seven sqlc input groups
are covered, including the internal transport group.

## Atomicity that currently keeps capabilities together

Food guest admission creates Core credentials and checks capacity/replay retention
alongside Food checkout data. Food ordering configuration occupies four columns
of Core's store row: `ordering_revision`, `accepting_orders`, `acceptance_mode`,
and `ordering_actor_id`. Its lock, update, and audit cannot simply become unrelated
remote calls. Before separate storage, move the Food-owned configuration and
replace guest admission with an explicit authorized operation and retry outcome.
The manifest bounds these exceptions by query, object, and access mode; column
scope still needs code review.

Food also calls Core's `PublicOfferingTx` while creating quotes and placing orders.
That helper locks the store/publication row inside the Food transaction; Core's
guest checks and replay wrapper participate too. These transitive accesses are
not additional foreign SQL inputs, but they are extraction constraints. A future
remote catalog boundary must preserve publication/availability validation and
retained agreed terms, not replace the lock with an unchecked stale read.

Food's development payment simulator reads and writes Payment state while holding
the Food order lock. Cancellation, simulated payment, and order events therefore
commit together. There is no external payment request inside that transaction.
This is a temporary simulator exception, not the production PSP integration design.
The merchant order list also joins the simulator projection.

Shopping order/stock/session changes and Lodging booking/allocation/session changes
record a Payment obligation through `RecordPayLater` in the same transaction.
The SQL belongs to Payment, so the query guard correctly does not report a foreign
write; nevertheless these callers still require the same database transaction.
Extracting Payment needs durable intent beside the domain decision and idempotent
delivery/reconciliation, not a network call while holding SQL locks.

Core publication saves visibility/generation and durable delivery intent together.
Discovery delivery happens later, outside that transaction. Core's privileged
`commerce.admin_orders` function reads Food and development Payment tables for the
authorized provider projection. Guest lookup functions also read Core credentials.
These database functions, foreign keys, row security, and notification triggers
remain shared-database couplings; declaring their owners does not eliminate them.

## Identity and application-interface audit

Payment subjects already carry domain, merchant, store, and immutable purchase UUID.
Human-readable order numbers are presentation identifiers, not payment identities.
Pay-later obligations use that scoped subject identity. This does not yet define
separate capture/refund operation identities for a production PSP; those need stable
operation IDs and retained results when that workflow is implemented.

Catalog effects retain a stable ID/key across retries and carry a fenced lease
generation. Catalog delivery uses store and catalog generation for its receiver
identity. A lease coordinates local attempts; neither identity nor a successful
claim proves exactly-once external execution.

Existing `sessionkit.Domain`, `payment.Terms`/`Subject`, and authorized Core
application operations are useful local seams. They are not all extraction-ready:
`sessionkit.Domain` and `RecordPayLater` accept SQL transactions; Food uses Core
principals/results; Discovery delivery consumes Core's persisted `Effect` type.
The notification-driven worker now uses small `Store` and `WakeSource` interfaces
and the owned `workmodel.Effect` type. The persistence alias preserves existing
callers without a duplicate implementation; Discovery's catalog payload still
depends on Core persistence types. A future remote
interface should carry authorized business inputs, stable operation identity, and
an outcome—not SQL transactions or generated persistence rows. Keep the existing
transaction participants local until their replacement preserves atomic intent.

## What the guards establish

Architecture tests discover named SQL inputs under `queries/`, identify references
to registered application objects, and require an exact exception for foreign
reads, writes, or function calls. New exceptions need a reason; unused and duplicate
exceptions fail. Migration-defined tables/functions require a declared owner.
New writes must use registered schema-qualified targets. Negative fixtures cover
CTE writes, quoted identifiers, comments/literals, unknown write targets/functions,
and unsupported write forms. Import guards also reject another owner's generated
SQL package, business-to-transport imports, and foreign commerce implementations.

These are conservative source checks, not a complete PostgreSQL parser or a
security boundary. They do not analyze function bodies, column changes, dynamic
SQL, unqualified reads/functions, nested comments, custom search paths, arbitrary
aliases matching schema names, or every PostgreSQL syntax extension. Existing
`FOR UPDATE` reads are recorded as reads, not as permission to extract their locks.
An existing exception permits editing that query's access to the same object;
transaction and tenant-isolation tests plus review remain necessary. Unsupported
SQL forms require extending the guard and its fixtures first. Generated Go remains
ignored; the existing handwritten-SQL-call guard protects the inspected runtime
packages, not an adversarial attempt to bypass it through another driver API.
