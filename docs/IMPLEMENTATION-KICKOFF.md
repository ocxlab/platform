# From the design to the first restaurant journey

20 October 2026, Europe/Berlin (UTC+02:00). Implementation handoff while the founder
reads the technical design. This is a delivery plan, not approval of every design
default or a record of business features already implemented.

The first useful result is a restaurant maintaining a menu, receiving an order,
and completing or resolving it with a customer. Build that connected path in small
increments. An operational scaffold provides repeatable tools; it does not replace
the commerce, network, and payment behavior established by the research.

## Foundation prepared now

The 21 October handoff in [FOUNDATION-DECISIONS.md](FOUNDATION-DECISIONS.md)
carries the founder's latest offering, resource, addressing, and fulfilment
clarifications and defines the bounded contract/persistence slice. Its current
acceptance record supersedes the original scaffold status below as work completes.

Task is the command interface, and Docker supplies compilers, runtime checks, and
local services. There is no CI configuration or production cloud dependency.
The Go module under `services/provider/` and PostgreSQL service leave room for a monorepo containing
frontend applications, contracts, and integrations without selecting their tools
prematurely. Runtime images run as non-root; local database administration is
explicitly a development convenience, not the application's future database role.

The original scaffold had no commerce persistence or business authentication.
The subsequent [foundation report](FOUNDATION-REPORT.md) records the completed
store/draft-offering contract, restricted grant, and PostgreSQL slice. Native ONIX,
UCP adapters, purchase execution, and payment remain unimplemented. Existing sandbox
evidence is reference material, not permission to copy secrets or experimental
implementations into this repository wholesale.

## Ownership to preserve as code is added

| Module or service responsibility | Records and decisions owned |
| --- | --- |
| Provider accounts | Merchant/store relationships, staff membership, customer profiles, account grants |
| Offering | Menu revisions, options, operating configuration, public publication intents |
| Food domain | Selection validation, serviceability, acceptance/fulfilment rules, capacity decisions |
| Purchase processing | Immutable terms, one purchase history, allocation, replay results, purchase action intents |
| Durable execution | Work claims, attempts, next execution and reconciliation times, preserving owner transactions |
| Payment service | Merchant/payee grants, financial operations, processor adapters and authenticated outcomes |
| Identity service | Durable merchant identity and authorized current endpoint/key bindings |
| Network integration and discovery | Native ONIX publication/crawl mapping, public search projection, peer scope and freshness |

The Food handler proposes domain changes; shared execution commits the purchase,
allocation, replay result, and pending actions together. Both commerce APIs use
that same path. Catalog edits use Offering instead; they do not invent purchases.
Payment owns a separate transaction boundary even if initially colocated.

Only create package boundaries when implementing their responsibility. The table
does not call for empty packages, compulsory microservices, or one database per
merchant. As UI applications arrive, they can live under `apps/`; versioned API
descriptions and generated-code conventions belong under `contracts/`; retained
composition belongs under `deploy/`. The provider Go module lives under
`services/provider/`; retained ONIX packaging/configuration lives under
`services/onix/`, separately from the owned implementation. Do not group all
backend runtimes into a single language directory or import upstream source into
the owned module without an actual reuse decision.

## Useful delivery milestones

| Milestone | Deliverable | Acceptance boundary |
| --- | --- | --- |
| F0: Local foundation | Docker/Task build, tests, API liveness, local PostgreSQL | Commands work without host Go/cloud credentials; no false commerce readiness |
| F1: Contract and persistence foundation | First versioned OpenAPI operations, generated artifacts, migrations, tenant-scoped access, Food handler boundary | Generation is reproducible; unauthorized and cross-merchant operations fail; real PostgreSQL integration tests use isolated test data |
| F2: First restaurant purchase | Merchant/store setup, editable/published menu, guest selection/terms/placement, staff view/actions, sandbox collection/refund, scheduled work | Pickup and own delivery; required extras; pause/auto/manual acceptance; immutable terms; replay/conflict; cancellation, expiry, and late-payment recovery |
| F3: Joined network slice | Two separately configured commerce providers, retained ONIX publication/crawl, compact discovery, one scoped payment service | App finds both merchants and resolves current authority; freshness/withdrawal, distinct payees, and wrong-host/merchant negative controls |
| F4: Deployment and release evidence | Selected managed hosting/recovery, operational checks, restaurant/device feedback, bounded Shopping/Lodging compatibility | Actual restore/rollback and declared limits; public security, legal/payment/material-rights checks; no launch promise based on liveness alone |

F0 and the bounded F1 foundation are now implemented; later milestones remain
planned acceptance requirements. See STATUS.md for checks actually executed.
Each later milestone should
leave a working path and tests, not a disconnected set of API stubs. Network
integration and shared payment are part of the implementation plan, not optional
features to rediscover after a single-provider storefront is finished.

## Decisions at the point they become necessary

Before F1, confirm the first concrete operation inventory, credential/grant
mechanism, OpenAPI/generator pairing, and migration/database tooling. These choices
can be evaluated locally; a production cloud decision is not needed to implement
tenant ownership and purchase transactions. Do not silently freeze a public UCP
version from the experimental sample version.

Before F2 payment integration, confirm the adapter/account model and merchant/payee
arrangements. Local development uses a deterministic test adapter before explicit
sandbox tests; successful fake outcomes must remain labelled test evidence.
Before taking real orders, tax, consent, cancellation, disclosures, production
payment eligibility, and restaurant workflow need qualified/domain checks.

Before F4, measure the complete selected package rather than estimating from the
Go API alone. Initial hosting relies on the managed provider's storage/recovery
facilities, with its retention and restore limits checked. A separately operated
protected copy, isolated backup store, and hybrid/multi-cloud availability are
later work, not compulsory launch infrastructure. The APIs/workers still retain
their decisions and unfinished work and stop rather than invent success when
records cannot be safely committed.

## Evidence and reuse discipline

The research baseline is sibling `ocp-lab` commit `878e242`, with document sections
2, 7–9, its internal claim map, and prior runtime reports providing the handoff.
Retain ONIX network seams and selected UCP helpers where useful, subject to the
exact materials' rights. Keep an implementation provenance manifest when actual
artifacts are adopted; currently none have been copied into this repository.

The implementation must re-establish its own acceptance results. Same-source
two-host tests are controlled integration, not independent interoperability. Update
the technical design after implementing the journey and documenting differences.
Provider transfer and independent stewardship remain later capabilities; stable
IDs, record ownership, and versioned contracts preserve their foundations now.
