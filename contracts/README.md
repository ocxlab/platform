# API contracts

Edit the modular OpenAPI source here, then generate its Go and TypeScript artifacts.
Requests and responses use JSON. YAML is only the readable authoring format.
This is the first owned administration contract, not an OCP standard or a claim
of UCP/Beckn conformance. It covers stores, draft revisions, publication,
provider-admin visibility control, public offerings, and guest Food quotes.

## Where definitions belong

```text
contracts/
  openapi/v1/
    openapi.yaml                Entry document and public component names
    paths/                      Operations grouped by resource path
    components/
      parameters/               Shared identifiers and Idempotency-Key
      responses/                Shared Problem Details and quote-capacity response
      headers/                  Shared request correlation
      security/                 Bearer credential requirements
    schemas/
      common/                   Identifiers, liveness, and Problem Details
      stores/                   Store resource and creation request
      offerings/                Model reference and immutable revision envelope
    domains/food/menu/v1/       Replaceable Food menu definition
    domains/food/selection/v1/  Selection and priced quote definitions
    examples/                   Fictional JSON documents checked in tests
  tooling/go.yaml               Pinned generator configuration
  build/                        Ignored, self-contained generated bundle
```

The entry document composes resources through local `$ref` references. Keep
shared concepts in their owning file rather than copying them into operations.
Group small, cohesive resource/request schemas in one file; split a file when
its contents have different owners or lifecycles, not just to maximize file count.
No remote references are fetched during generation. Files outside the v1 root
are rejected. Dependencies and image digests are pinned separately from API versions.

## Change and verify a contract

```text
task contracts:generate
task web:generate
task fmt
task check
```

Generation validates source references, bundles them, reloads the serialized bundle
as a fresh consumer, and validates it again. Generated models, client, server
interfaces/routing, and embedded contract live in
`services/provider/internal/httpapi/generated.go`. Commit that file with the source
change; never hand-edit it. `task contracts:check` regenerates to a temporary file
and compares it without rewriting committed Go. TypeScript types live in
`apps/web/src/lib/api/generated.d.ts`; `task web:check` checks their drift and
consumers. Both generators use the same rebuildable, ignored bundle.

`task check` also verifies fictional examples, Food definition rules, runtime
responses, generated-client round trips, authorization, retries, and real PostgreSQL
behavior. The integration suite uses unique disposable databases, not the development
database. Go tests without `OCP_TEST_DATABASE_URL` explicitly skip that suite;
`task test:integration` supplies it. A passing unit-only run is not DB acceptance.

The API is `/api/v1`; its current draft document version is `0.1.0`. Changes in
meaning or removal require a new major contract. Optional additions must still
respect declared compatibility: closed request schemas deliberately reject unknown
fields. A new optional request field is not usable against an older server unless
negotiated or otherwise supported. Offering model `ocp.food.menu` version `1.0.0`
is independent of the API version. Append immutable revisions with an expected
`base_revision`; do not reinterpret retained documents after model upgrades.

All failures use `application/problem+json`, with a stable `code` and safe details.
Private mutation retries require a 16–128 character `Idempotency-Key`. Matching requests
within the audience/merchant/grant/operation/resource scope return the original successful
status and byte-identical body; changed content returns 409. Fingerprints use
validated, decoded JSON, not whitespace or key order. Results and mutations commit
together. Successful keys have no automatic expiry in this foundation; cleanup
and key reuse require an explicit future contract. A grant is checked again on
every retry. Unknown offering models return 422, not optimistic acceptance.
Revisit retention/archival before general availability; do not enable unbounded
merchant write volume without that explicit operational and contract decision.

Each operation declares `x-access` as public, merchant, provider, or guest. Public operations
also explicitly clear security; private operations use the matching bearer scheme
and an `x-required-scope`. Startup rejects inconsistent audiences or unknown/missing
actions. These local extensions describe opaque grants, not OAuth scope negotiation.
Every response carries a fresh `X-Request-ID`; Problem Details `instance` carries
the matching UUID URN. `code`, not the generic `about:blank` type, is v1's machine
discriminator. Expected errors are listed on each operation using the shared
Problem response. Neither request bodies nor credentials are logged.

## What generated structure does not implement

Generated types and schema validation are only the structural boundary. Application
code still owns authorization, tenant scope, semantic validation, atomic revision
updates, and replay. For example, the Food schema bounds option counts; its
validator additionally checks that a required choice can actually be satisfied
and that identifiers are unique within their scope. Installing a schema file
alone does not install a domain's behavior.

Merchant commerce, Customer commerce, Merchant account, and Customer account remain
distinct API surfaces. This increment implements Merchant account store setup,
Merchant commerce offering administration, public Food quotes, and separate
provider publication administration. Identity, discovery, and payment
keep their own owners; these boundaries do not require separate servers. Native
ONIX integration and selected UCP capabilities remain planned connected slices.
Before importing their schemas, record exact versions, adopted capabilities, and
material-specific reuse rights rather than copying an ecosystem wholesale.

## Managed store listing

`GET /api/v1/merchants/{merchant_id}/stores` requires the merchant's `store:read`
grant and returns private managed stores, including unpublished stores. The web
workspace uses this same API to offer store-name links instead of ID entry.
The table shows latest draft item counts, revision numbers, and update times.
These summaries are authorized store-management metadata under `store:read`;
full offering documents still require `offering:read`. Counts exclude variants
and extras, are zero before the first save, and are null for unsupported models.
Update times reflect the latest draft save (or store creation), not publication
or moderation. `StorePage` uses `ManagedStore`, reusing Store property definitions;
pagination parameters
are separate reusable definitions. The default page size is 50, capped at 100.
Pass `next_after` as `after` to continue in ascending immutable store-ID order.
These are live pages, not a multi-request snapshot; return to the first page to
include stores created before the current cursor. A cursor does not confer access
to another merchant's stores. An empty list is `items: []`, not an error or null.

## Publication and quote behavior

`GET /api/v1/public/stores` supplies the homepage's anonymous local directory.
It exposes only names and merchant/store identifiers for visible publications,
not drafts, suspended stores, account data, or private operational records. A
narrow migration-owned SQL function provides that cross-tenant projection while
ordinary table queries retain tenant RLS. Its trusted owner must have the database
privileges needed for that projection; never transfer ownership to the runtime.

The optional `q` is a case-insensitive literal name substring, trimmed before
matching. Pages default to 50, cap at 100, and use a query-bound continuation
in ascending merchant/store ID order. Keep `q` with `next_after`; omit the cursor
for a new search. Changed-query cursors return 400. These are live pages, not a
snapshot. This convenience directory does not implement ONIX or network-wide
discovery. Edge rate limits and search scaling remain public-release work;
bounded results alone do not bound database scan cost or request frequency.

Publication selects an exact immutable revision and advances a separate generation.
Administrative suspension preserves that choice; merchant publishing cannot undo
suspension. Both use expected generations and attributable atomic audit records.
Public responses contain model-owned projections, not private draft history.

Guest quote creation is anonymous and deliberately non-idempotent: each successful
POST creates a fresh ten-minute snapshot, with no purchase, reservation, acceptance,
payment, or kitchen effect. The server validates and prices selections against the
locked, visible publication. Stale generations return 409. Quote IDs alone must
not become purchase authority in a later placement contract.

The local storage guard allows at most 1,000 active quotes per store; 429 includes
`Retry-After: 60`. Creation prunes up to 1,000 expired snapshots for that store;
expiry is a validity limit, not a deletion guarantee. This is not internet rate
limiting: an attacker could occupy those slots. Public release requires separately
tested abuse controls, appropriate limits, and a retention policy.

## Guest checkout

Checkout uses a GuestBearerAuth audience and guest-owned
quotes. Anonymous preview quotes do not confer placement authority. Pickup
placement retains agreed quote terms, pending acceptance, without payment or
preparation. Quote expiry limits first placement; it does not expire an order.
Guest credentials are merchant/store scoped and expire after seven days. Receipt
identifiers alone do not authorize reads. Placement and its event/replay commit
together; a retry retains the original terms even after quote expiry or withdrawal.
The web retains one checkout intent per store, including its original retry key.
See `docs/STATUS.md` for executed checks and remaining product boundaries.

## Restaurant decisions and guest cancellation

The merchant Food order API provides a newest-first inbox, individual reads,
and explicit accept/reject decisions. Its operations require `order:read` or
`order:manage`; a catalog-writing grant alone does not confer order access.
The placing guest can cancel an open, unprepared order, including after acceptance.
The development adapter also supports cancellation during simulated checkout and
an atomic simulated refund after simulated payment. These are not real-processor
refund guarantees. Separate provider `order:inspect` authority permits
read-only inspection through the administration API, not merchant impersonation.

`expected_revision` refers to operational progress, not the offering revision
inside agreed terms. A stale action returns `order_revision_conflict` with
`current_order_revision`; read the current order before choosing another action.
Invalid state transitions return `order_transition_conflict`. A successful
decision or cancellation returns 201. Revision, attributable event, and original
replay result commit together. A matching successful retry returns its historical
body, while GET returns current progress and ordered history. Closed orders cannot
be reopened by later acceptance. Acceptance neither collects money nor starts
preparation. The explicitly enabled development-payment operation below adds
simulated financial progress; kitchen progress remains unimplemented.

Placement bodies and agreed terms remain immutable. Migration 00004 backfills a
separate operational row for existing orders. Current reads supply `revision`;
it remains optional in the response schema so retained placement replay bodies
without it stay valid. `history` is omitted from inbox pages and old placement
replays. The bounded development checkout permits ten attempts, with at most 24
history entries in the current response schema. Extend the history contract or
add pagination before adding further events. Expanded state enums require updated consumers: this is a
coordinated draft-contract increment, not proof that older generated clients
understand new lifecycle states.

Inbox pages contain at most 100 orders (default 50), ordered by placement time
descending and immutable order ID descending for ties. Pass opaque `next_after`
unchanged as `after`. A cursor is a position, not a credential. Refresh the first
page for new arrivals; multi-request pagination is not a snapshot. The UI uses
explicit refresh, not push notifications or a realtime delivery guarantee.

Fresh development credentials include their audience's order scopes.
`task dev:order-access` explicitly extends existing active full-merchant and
publication-admin walkthrough grants in the local `ocp_dev` database. It is not
part of application migrations and must not become a production privilege upgrade.

Stable machine errors are owned by module under [errors/](errors/README.md).
Run `task errors:generate` after catalog changes. English, German, and Arabic Fluent
resources live with the web application; API problem details remain diagnostic,
not the source of customer-facing translations.

## Development payments and readable references

`paths/development-payment.yaml` owns the guest-only simulation operation, and
`schemas/payments/development.yaml` owns its reusable request. It uses the same
guest scope, expected revision, and idempotency contract as order changes. The
backend rejects this operation unless explicitly enabled; local composition
enables it. Clients cannot supply amounts or currency: immutable agreed terms
supply both. No real processor credentials, funds, or webhook claims are involved.
The payment-provider identifier is extensible; only `development` is implemented.
Real adapters require their own settlement, cancellation, and refund behavior.

Store `handle` and order `reference` are optional presentation fields so retained
historical response bodies remain valid. Current reads overlay persistent,
provider-unique sequence values; UUIDs continue to define API identity and scope.
See [the walkthrough](../docs/LOCAL-PAYMENTS-AND-REFERENCES.md).

## Tooling note

The pinned kin-openapi v0.133.0 default internal-reference naming resolver looped on
an absolute path during initial generation. The small bundling command supplies
a deterministic local-path resolver instead. It materializes named root aliases
before internalizing references and validates the serialized result, including
reloading the generated embedded contract in tests. Resolver behavior is tooling,
not a new wire-format convention. Re-evaluate this workaround with a cohort upgrade.
