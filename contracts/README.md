# API contracts

Edit the modular OpenAPI source here, then generate its Go and TypeScript artifacts.
Requests and responses use JSON. YAML is only the readable authoring format.
This is the first owned administration contract, not an OCP standard or a claim
of UCP/Beckn conformance. It covers stores, draft revisions, publication,
provider-admin visibility control, public offerings, and guest Food quotes.

## Where definitions belong

The experimental [merchant MCP contract](mcp/v1/merchant/README.md)
is maintained separately from REST. It documents task-oriented tool semantics;
the pinned SDK derives their JSON Schemas from typed inputs. It is not an
automatic OpenAPI-to-tool export or a replacement for these REST definitions.

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

`task contracts:lint` runs pinned Spectral 6.17.0 and its rule regression tests
inside Docker, separately from frontend dependencies. `task check` includes it.
Edit the root [`.spectral.yaml`](../.spectral.yaml), not an editor-only configuration.
The linter follows local references from the modular entry document, so findings
point to owning source files rather than only to the generated bundle. Warnings
fail the check; there is no blanket legacy-warning baseline.

The rules start with `extends: [[spectral:oas, all]]` and add these owned-API policies:

- Literal path segments use kebab case, without trailing slashes, HTTP/action verb
  segments, or embedded query strings. Search/filter/pagination parameters are
  declared normally, not banned.
- Path and query names use lower camelCase. Standard header names such as
  `Idempotency-Key` retain their HTTP spelling. JSON property names remain the
  existing snake_case contract; this is not a payload-format migration.
- Request bodies use `application/json` with an object root, including composed
  schemas that constrain the root to objects. Put batch arrays inside an object.
  GET operations have no request body. Direct signed object-storage uploads are
  a separate storage interface, not exceptions to the business REST contract.
- Error responses (4xx, 5xx, wildcard error ranges, and `default`) declare only
  `application/problem+json` with a schema. Redirects are not errors: signed-image
  redirects may omit bodies, and successful SSE streams use `text/event-stream`.
- Operations declare their access audience. Standard OAS rules check unique
  operation IDs, valid parameters/references, examples, descriptions, and tags.
  Runtime authorization, recognized scopes, replay, and actual error bodies still
  require the existing Go and integration checks; lint cannot establish behavior.

Four explicit baseline exceptions are documented beside their switches: private
drafts have no granted public license/URL to claim, the maintainer has no published
support email/URL, and stable root code-generation aliases may appear unused to
source-level analysis. Do not invent metadata or remove exported names to silence
these findings. Revisit those exceptions when the API is published.

Configuration follows Spectral's [ruleset](https://github.com/stoplightio/spectral/blob/develop/docs/getting-started/3-rulesets.md)
and [custom-function guidance](https://github.com/stoplightio/spectral/blob/develop/docs/guides/5-custom-functions.md),
consulted 22 October 2026; the installed CLI and transitive dependencies are pinned
in `tooling/package.json` and `tooling/pnpm-lock.yaml`.

The camelCase placeholder migration updates the generated Go/TypeScript contracts,
tenant-matching middleware, and client parameter objects together. Actual URL
segments, query names, stored JSON, and persisted replay bodies are unchanged.

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

`GET /api/v1/merchants/{merchantId}/stores` requires the merchant's `store:read`
grant and returns private managed stores, including unpublished stores. The web
workspace uses this same API to offer store-name links instead of ID entry.
The table shows latest draft item counts, revision numbers, and update times.
These summaries are authorized store-management metadata under `store:read`;
full offering documents still require `offering:read`. The optional `handle`
query performs an exact lookup of a readable store handle within the same
authorized merchant; unknown or foreign handles
return an empty page. It does not replace UUID identity in other API operations.
Counts exclude variants and extras, are zero before the first save, and are null
for unsupported models.
Update times reflect the latest draft save (or store creation), not publication
or moderation. `StorePage` uses `ManagedStore`, reusing Store property definitions;
pagination parameters
are separate reusable definitions. The default page size is 50, capped at 100.
Pass `next_after` as `after` to continue in ascending immutable store-ID order.
These are live pages, not a multi-request snapshot; return to the first page to
include stores created before the current cursor. A cursor does not confer access
to another merchant's stores. An empty list is `items: []`, not an error or null.

## Publication and quote behavior

The coordinated draft Food menu increment adds optional ordered `categories`
(at most 50) and item `category_id` references. Category identifiers must be
unique and all references must resolve; older menus without categories remain
valid. Categories and assignments are explicitly included in the public
projection. They organize browsing and do not change pricing, availability, or
agreed purchase terms. An uncategorized item remains visible. These additions
require updated servers/clients because the old menu schema was closed; retaining
the draft model label does not establish compatibility with old validators.

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

### Basket notes and localized Food menus

The coordinated draft menu addition supports optional `translations` for category,
item, variant, option-group, and option names, limited to `de`, `en`, and `ar`.
Default `name` remains required. Selection requests may supply `locale` for quote
labels, with fallback to the default; identifiers and amounts are unchanged.
Optional `default_option_ids` on option groups must resolve uniquely and, when
nonempty, satisfy the group's selection limits. Defaults are suggestions for new
selections, not replacements for the customer's previously chosen options.

Selection-line `note` and request/quote `note` carry optional plain-text requests
of at most 500 Unicode characters. Notes are retained in agreed terms and shown
to the merchant; they do not change pricing or guarantee special preparation.
The selection endpoint already supports multiple lines. Browser previews use
that same guest API; automatic checks are not an independent purchase authority.
These additions require coordinated client/server updates because older schemas
were closed, despite retaining the current draft model version label.

### Public availability updates

`GET /api/v1/public/merchants/{merchantId}/stores/{storeId}/offering-updates`
is a public SSE hint stream. Clients re-read both the published offering and Food
ordering state on connection/reconnection and matching changes. Publication and
ordering revisions generate commit-time notifications; drafts and order events
do not. The public listener uses a separate PostgreSQL channel from private order
updates. An already subscribed viewer receives a withdrawal hint before closure.
This is not event replay, a reservation, or permission to bypass placement checks.

Each API process uses one additional LISTEN connection, not one per browser.
Public streams are bounded to 1,000 total and 32 per direct network peer, with
ten-second transport heartbeats and a two-minute lifetime. Forwarded headers are
not trusted. A reverse proxy currently shares its peer allowance; a production
trusted-proxy/admission policy and internet abuse controls remain release work.
SvelteKit forwards the fixed public path without credentials. Hidden pages close
the stream; reconnect forces a fresh JSON read instead of replay or state polling.

Readable web navigation uses `/stores/{name}-{handle}/orders/{reference}`.
`GET /api/v1/public/stores/{storeHandle}` resolves one visible store through
an indexed, narrowly authorized public projection; it never exposes private,
withdrawn, or suspended stores. The guest-only `food/orders/by-reference/{orderReference}`
read has the same store scope and guest ownership checks as the UUID read and
does not depend on publication. Other operations continue to use UUIDs.
The web retains a small HttpOnly store-routing hint alongside the scoped guest
credential. This hint supplies identifiers, not authority; the API checks every
read and mutation. Cookies cover `/stores` so renaming a shop does not lose an
existing guest session. Custom domains and cross-site processor returns remain
separate work. Previous pre-launch UUID web routes are not preserved.

Checkout uses a GuestBearerAuth audience and guest-owned
quotes. Anonymous preview quotes do not confer placement authority. Pickup
placement retains agreed quote terms, using the merchant's acceptance mode, without payment or
preparation. Quote expiry limits first placement; it does not expire an order.
Guest credentials are merchant/store scoped and expire after seven days. Receipt
identifiers alone do not authorize reads. Placement and its event/replay commit
together; a retry retains the original terms even after quote expiry or withdrawal.
The web retains one checkout intent per store, including its original retry key.
See `docs/STATUS.md` for executed checks and remaining product boundaries.

## Restaurant decisions and guest cancellation

Store ordering settings have their own Food resource:
`GET/PUT /api/v1/merchants/{merchantId}/stores/{storeId}/food/ordering`.
Read requires `order:read`; changes require `order:manage`, an expected settings
revision, and an idempotency key. Defaults preserve existing behavior: open and
manual acceptance. Pause/resume is independent of publication. Pausing blocks
new quotes and first placements, including previously quoted selections, but
does not change existing orders or successful placement replays. The public GET
under `/api/v1/public/merchants/…/food/ordering` exposes settings only for visible
stores, never the authorizing merchant grant or audit.

Changes, actor/request audit, and replay commit atomically. Settings, publication,
quotes, and placement serialize on the same store row. Automatic acceptance uses
the merchant policy current at placement, records acceptance under the grant
that enabled it, and leaves payment and preparation untouched. Later setting
changes do not rewrite prior orders. Timed reopening and live availability hints
are not part of this increment; current reads are advisory and writes recheck.

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
simulated financial progress; the merchant fulfilment operation separately
records preparation and pickup progress.

Placement bodies and agreed terms remain immutable. Migration 00004 backfills a
separate operational row for existing orders. Current reads supply `revision`;
it remains optional in the response schema so retained placement replay bodies
without it stay valid. `history` is omitted from inbox pages and old placement
replays. The bounded development checkout permits ten attempts, with at most 32
history entries in the current response schema. Extend the history contract or
add pagination before adding further events. Expanded state enums require updated consumers: this is a
coordinated draft-contract increment, not proof that older generated clients
understand new lifecycle states.

Inbox pages contain at most 100 orders (default 50), ordered by placement time
descending and immutable order ID descending for ties. Pass opaque `next_after`
unchanged as `after`. A cursor is a position, not a credential. Refresh the first
page for new arrivals; multi-request pagination is not a snapshot. The UI adds
optional SSE invalidation hints, not an event-history replay or a realtime delivery
guarantee. See [live status design](../docs/LIVE-ORDER-UPDATES.md) for the modular
stream contracts, authorized reads, reconnect behavior, and resource limits.

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

## Merchant pickup progress

`paths/food-order-fulfilment.yaml` defines the merchant `order:manage` operation.
Its reusable `FoodFulfilmentRequest` lives beside the Food order schemas. The
same operation serves the merchant UI and API consumers. This slice requires
accepted, open, paid pickup orders and advances `fulfilment_progress` from
`not_started` to `preparing`, `ready`, then `collected`. Reports cannot skip or
reverse stages. Payment and acceptance do not start kitchen work automatically.

Progress, attributable history, and the successful replay response commit under
the same operational lock as cancellation and simulated payment. A stale revision
conflicts; successful retries return their historical response. Current detail
and inbox reads overlay persisted progress without rewriting agreed terms or
legacy placement bodies. The history bound is 32 entries for the current bounded
attempt/transition set. Expanded enums require coordinated draft-client updates.

`collected` is terminal fulfilment, not a change to the separate resolution axis.
Automatic guest cancellation ends when preparation starts; that is a capability
limit, not a statement about statutory or support rights. Merchant reports are
not independent physical proof. Own delivery, cash payment conditions,
preparation deadlines, and post-preparation support/refunds remain
separate work. No ingredient inventory is required by this progression.

## Provider administration browsing

`GET /api/v1/admin/stores` uses `publication:read`; `GET /api/v1/admin/food/orders`
uses `order:inspect`. Both require the separate provider credential audience and
return bounded summaries across this provider's merchants. Store results include
unpublished and suspended stores and identify the owning merchant by name. Order
results omit agreed terms, guest authority, and payment details; existing scoped
detail reads serve deliberate inspection. No order write authority is added.

The shared query and continuation parameters live under `components/parameters/`;
summary/page schemas live under `schemas/admin/`. Literal substring search and
status filtering precede pagination. Pages default to 25 and cap at 100, ordered
by immutable display sequence descending. Sequence order is not a strict commit
timestamp ordering. Cursors bind resource, trimmed query, and status, not page
size or grant identity; every request authenticates and authorizes again. Live
pages are not snapshots. Return to the first page for new records. Draft update
times do not represent publication/moderation update times.

Migration 9 exposes only narrow read-only projection functions with fixed search
paths, no PUBLIC execution, and a fresh provider-grant scope/expiry/revocation check.
It neither gives the runtime a cross-tenant table-read role nor weakens tenant RLS.
Query size and page size are bounded; production request throttling and larger-
dataset search plans remain release work, not something pagination alone solves.

## Tooling note

The pinned kin-openapi v0.133.0 default internal-reference naming resolver looped on
an absolute path during initial generation. The small bundling command supplies
a deterministic local-path resolver instead. It materializes named root aliases
before internalizing references and validates the serialized result, including
reloading the generated embedded contract in tests. Resolver behavior is tooling,
not a new wire-format convention. Re-evaluate this workaround with a cohort upgrade.
