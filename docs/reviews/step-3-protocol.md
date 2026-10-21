# Step 3 protocol/security design note

**Reviewer perspective:** protocol/contracts

**Reviewed:** 21 October 2026 (Europe/Berlin, UTC+02:00)
**Snapshot:** pre-implementation design proposal for publication, public browse,
guest Food selection, and provider administration.

## Bounded verdict

The proposed slice has the right boundary: private merchant drafts, an explicit
publication decision, a public projection, and selection terms are three different
things. Keep them separate. It is also correct that selection creates neither a
purchase nor a reservation, and that provider suspension affects public exposure
without deleting merchant work or impersonating a merchant publication decision.

Proceed with the slice after representing the minimum semantics below in the OpenAPI
source and transaction design. None requires production login, provider transfer,
ONIX publication, payment, a customer account, or an all-purpose product model.

## Essential contract semantics

### Publication is an atomic public-state transition

- Merchant publication mutation takes an exact retained `revision` to publish, or an
  explicit `withdraw` action; it must not mean “publish my latest draft.”
- It requires `expected_publication_generation` and an `Idempotency-Key`. A successful
  publish, replace, withdraw, or provider suspension-state change advances the public
  generation. A stale expected generation returns `409` with a safe current generation.
- In one transaction, verify that the requested revision belongs to the authorized
  merchant/store, is a supported installed model, and can produce a public projection;
  persist a model-owned immutable projection/snapshot plus its source revision and
  model/version. Do not expose the raw draft document just because Food v1 happens to
  share fields with its public representation.
- The public projection must contain only data needed for browsing/selection. It must
  exclude grants, staff data, internal availability notes, audit fields, private URLs,
  and future draft-only metadata by construction.

### Public reads must be deliberately public

- Each public operation uses `security: []`, has an explicit access/audience extension
  (for example `x-access: public`), and is allowlisted at startup. Conversely, every
  non-public operation must declare its required scope and audience; startup fails
  closed on an unclassified operation. Do not create an authentication bypass based on
  path prefixes.
- `GET /api/v1/public/merchants/{merchant_id}/stores/{store_id}/offering` returns only
  the current active projection, its public generation, source model/version, and
  public revision identifier as necessary for later terms. It returns `404` when no
  active public offering exists, including a provider-suspended store. Do not disclose
  a suspension reason or distinguish it publicly from withdrawal.
- Decide caching deliberately. A short `ETag` based on public generation can reduce
  catalog traffic, but cache validation never authorizes a selection; the selection
  transaction is authoritative. If no caching is implemented yet, say so rather than
  copying the private `no-store` rule without analysis.

### A selection is immutable, expiring quoted intent—not an order

- Guest selection request identifies the exact `publication_generation` it was built
  from and a list of item/variant/option identifiers. Do not accept any client price,
  currency, display name, availability assertion, or draft revision as authoritative.
- In one transaction, load/lock or compare the current public state, reject stale,
  withdrawn, and suspended generations, then validate the chosen Food combination:
  item and variant exist, item is manually available, option groups meet min/max
  rules, options belong to their group, and identifiers are not duplicated. Calculate
  all minor-unit integer line prices and the total from the stored projection.
- Persist immutable terms containing at least opaque selection ID, merchant/store,
  public generation, source revision/model/version, normalized chosen identifiers,
  currency, calculated line/total values, creation time, and expiry. Return those
  terms to the guest. State in the schema and descriptions that `selection_id` is not
  a purchase, booking, reservation, acceptance, payment authorization, or delivery
  promise.
- This is a guest mutation. Require `Idempotency-Key` with a documented scope and a
  canonical request fingerprint, or explicitly accept duplicate records. Prefer the
  former. With no customer account, scope it to a server-issued anonymous capability
  or other bounded client/session identifier—not a raw IP address—and keep its
  implementation separate from future customer identity.
- Define expiry behavior: an expired selection is not usable by a later purchase
  operation; selection creation after a changed generation returns `409` (for example
  `stale_publication`) with a safe current generation when that information is public.
  Retention/cleanup of expired records is an operational decision; do not silently
  turn expiry into deletion of audit evidence.

### Provider administration remains a separate authority

- `/api/v1/admin/...` uses a separate credential audience/lookup and explicit
  provider-admin action scopes. It must not accept a merchant bearer merely because
  that bearer happens to name the same merchant or store. Keep local founder
  provisioning development-only.
- Admin `GET` may expose internal publication/suspension state to authorized provider
  staff. Its public counterpart must not. A suspension mutation must atomically update
  the public-state guard/generation and append attributable audit data in the same
  database transaction.
- Audit entries need immutable actor audience and grant/credential identifier, action,
  merchant/store targets, before/after suspension state or a bounded change summary,
  timestamp, and request identifier. Do not record credential material, request bodies,
  or a free-form private suspension reason in public projections.
- Suspension means “do not serve public projection or issue a new selection.” It does
  not overwrite the merchant's publication record, delete drafts, or decide later
  purchase/refund rules. Those future actions must state their own treatment of an
  existing selection.

## Required response and error clarity

Use the existing Problem Details shape and request-ID contract. At minimum, declare
and test: `400` malformed request, `401` invalid credential on private/admin routes,
`403` valid but wrong audience/scope, `404` absent or non-public resource, `409`
publication-generation conflict/stale selection, `413` body bound, `415` media type,
`422` invalid Food selection or unsupported model, and `503` persistence unavailable.
Keep a stable `code` for the client distinction (`revision_conflict`,
`stale_publication`, `invalid_selection`, `unsupported_model`, and so on); preserve
the current `X-Request-ID`/Problem instance behavior.

## Tests that establish this slice

- Two concurrent merchant publication mutations from the same expected generation:
  one succeeds, one receives the documented conflict; repeated matching mutation
  returns the original response.
- Public read never returns draft-only fields. Withdrawn and suspended stores both
  behave as absent to public reads; merchant drafts remain readable to authorized
  merchant operations; admin view/audit distinguishes the state.
- A selection from a fresh published generation returns retained terms with only
  server prices. A concurrent/new generation or suspension cannot yield terms tied to
  a stale projection.
- Invalid Food combinations, unavailable items, repeated option IDs, and client price
  injection are rejected. No test should assert a payment, reservation, purchase, or
  acceptance side effect.
- Merchant token cannot invoke admin operations; admin token cannot be treated as a
  merchant token; a public endpoint requires neither. Startup rejects an unclassified
  endpoint.
- SvelteKit/UI clients use generated TypeScript from the same versioned OpenAPI source;
  no browser-side rule is treated as authoritative.

## Implementation limits retained

This note does not choose production authentication, customer login, rate-limiting
policy, actual custom-domain routing, ONIX publication, payment, order placement,
reservation/capacity allocation, or external fulfilment. Those are separate slices.

## Implementation review — 21 October 2026

**Snapshot reviewed:** uncommitted publication/browse/guest-selection/provider-admin
implementation, including modular OpenAPI source, PostgreSQL migration, Go handlers,
selection logic, integration tests, and the SvelteKit server-side API boundary. The
implementation owner reported an earlier PostgreSQL integration pass and current
web checks in progress. This review did not run shared browser or test-database tasks.

### What is correctly realized

- Publication is a store-row-serialized state transition with an expected generation,
  an exact immutable source revision, model-owned projection, and atomic audit/replay
  result (`internal/persistence/publication.go:52-104`). `public_document` is stored
  as a snapshot rather than exposed as a raw draft.
- Public offering reads join only an active, unsuspended selected revision and return
  `404` for absent/withdrawn/suspended state (`publication.go:117-141`). The Food
  projection enumerates model fields rather than forwarding unlisted draft fields
  (`internal/offering/food/selection.go:12-20`).
- Quote creation locks the same store row as publication changes, compares the public
  generation before calculation, persists immutable ten-minute terms, and uses only
  server-side integer prices (`publication.go:162-207`, `selection.go:40-120`).
  The deliberately non-idempotent guest operation is explicitly labelled as such and
  has no purchase/reservation/payment side effect.
- Provider grants have a distinct opaque-token prefix and lookup table, while merchant
  grants cannot enter admin routes (`provider_auth.go:16-46`,
  `httpapi/server.go:82-113`). Runtime SQL privileges keep audit records immutable to
  the application role and provider grants unreadable by it (`00002_publication.sql`).
- The SvelteKit app calls the same generated OpenAPI TypeScript interfaces through
  server-only clients. Temporary bearer credentials live in server process memory and
  HTTP-only, Strict SameSite opaque session cookies—not page data or browser storage
  (`apps/web/src/lib/server/api.ts`, `sessions.ts`). This is appropriately labelled
  development bootstrap rather than production authentication.

### Findings to address

| Severity | Evidence | Required correction or disposition |
| --- | --- | --- |
| **Blocker** | `internal/httpapi/server.go:46-69`; `openapi.yaml:21-41`; existing merchant operations lack `x-access`, and public allowlisting checks only `security: []` | The declared policy is that every operation has an explicit audience and that startup fails closed on an unclassified operation. The code safely uses a static public allowlist, but does not verify `x-access: public`, does not require `x-access: merchant` for private merchant operations, and does not verify an operation's OpenAPI security scheme against its audience. Add explicit `x-access` to every operation, validate a recognized `public`/`merchant`/`provider` classification at startup, validate `security: []` only for public and `ProviderBearerAuth` for provider operations, then select authentication from the verified classification. This is contract/security correctness, not an authentication-system expansion. |
| High, pre-public-release | `publication.go:180-192`; `paths/food-quotes.yaml:8-10`; `docs/PUBLICATION-SLICE.md:47-50` | The 1,000-active-quote cap bounds storage, but a trivial anonymous script can fill all slots for a store for ten minutes and deny legitimate guests. It is **not** rate limiting. The loopback-only local slice can retain this explicit limitation, but do not accept it as a public-internet control. Before public exposure, put the quote endpoint behind a deliberate provider/reverse-proxy or application rate-limit policy with trusted-client/proxy assumptions, and test `429` behavior. |
| High | `00002_publication.sql:30-43`; `publication.go:102-104`; `httpapi/server.go:71-76` | The provider audit has actor, action, reason, target, generation, and timestamp, but not the request identifier promised as part of attributable audit in this review's design requirements. Carry the request ID through request context and write it in the immutable audit row in the same transaction. This is a small schema/handler change and keeps operator support evidence connected to the response a caller received. |
| Medium | `paths/food-quotes.yaml:19-27` | `CreateFoodQuote` is explicitly anonymous and non-idempotent, but its `201` description says “authenticated mutation retries return the original successful body.” Replace it with language matching the deliberate behavior: each successful submission creates a distinct quote and a retry may create another quote, with no commercial side effect. |
| Medium | `httpapi/server.go:189-195`; all `429` response declarations use the generic Problem component | The runtime emits `Retry-After: 60` for quote capacity, but the OpenAPI `429` response does not declare that header. Add a reusable 429 response/header component or declare `Retry-After` on each response where emitted, and response-validate it. |
| Medium | `internal/integration/publication_test.go:83-108,143-179` | Existing tests cover sequential stale generations and concurrent publication writes. Add an explicit concurrent quote-versus-publish/suspend test proving a quote cannot commit against a publication state changed in the same interval. The SQL row-lock design appears sound, but this claim should be demonstrated. |
| Low | `httpapi/server.go:189-190`; `schemas/common/problem.yaml:23-27` | A stale public selection returns generic `publication_conflict`, though clients benefit from knowing they must refresh a public offering. Either retain that documented generic code consistently or map quote-context conflicts to `stale_publication`; do not claim a distinct code without emitting it. |

### Protocol verdict

The data/behavior boundary is otherwise sound for this bounded local slice. Fix the
audience-classification blocker, then re-run contract generation and the relevant
integration checks. The quote-capacity and audit-correlation items are important
before a publicly reachable pilot; they do not require adding payments, customer
accounts, ONIX, or provider transfer now.

## Verification update — 21 October 2026

Static re-review confirms that the two material findings above have been corrected.

- Every operation now carries the explicit `x-access` audience expected by the
  startup policy. Startup checks the declared audience against the recognized public
  and provider operation allowlists, requires an explicit anonymous security override
  only for public operations, and validates the effective bearer scheme for every
  protected operation (`internal/httpapi/server.go:46-103`). This covers the earlier
  merchant endpoints as well as the new public, provider, and liveness endpoints.
- The publication audit now persists the UUID response/request identifier in the same
  transaction as the provider suspension mutation
  (`migrations/00002_publication.sql:30-43`,
  `internal/persistence/publication.go:52-104`). This restores the promised support
  correlation without exposing credentials or private reasons.

### Remaining bounded corrections

These are compact contract/test corrections, not reasons to broaden the slice:

1. `paths/food-quotes.yaml:19-20` still describes the `201` response as if an
   authenticated mutation retry returns its original response. `CreateFoodQuote` is
   expressly anonymous and non-idempotent; the description must instead say that a
   successful repeat can create another quote with no commercial side effect.
2. Runtime sends `Retry-After: 60` when the local quote-capacity guard is reached,
   but the OpenAPI `429` response does not declare that header. Declare the reusable
   header on that response and contract-test it, or cease emitting it.
3. The shared store-row lock makes quote-versus-publication serialization credible,
   but add a direct concurrent quote-versus-publish/suspend integration case as
   acceptance evidence before calling the state invariant demonstrated.

The 1,000-active-quote guard remains a local storage bound, not public-internet abuse
protection. The documented loopback-only gate and pre-release rate-limit requirement
are an appropriate limitation for this slice.

### Updated protocol verdict

**No remaining blocker** is visible in the statically reviewed bounded local design:
public/private/provider audience separation and attributable provider audit are now
implemented coherently. Resolve the two OpenAPI clarity mismatches and retain the
concurrency case as the next focused test; confirm the implementation owner's pending
generation and integration checks before accepting the slice.

## Final static verification — 21 October 2026

The remaining bounded corrections are now present in the reviewed source.

- The `201` description now accurately says that repeating an anonymous quote request
  creates another immutable quote without commercial effects
  (`paths/food-quotes.yaml:19-20`).
- `429` now uses a dedicated reusable response with both `X-Request-ID` and the
  integer `Retry-After: 60` contract (`components/responses/quote-capacity.yaml`),
  and the integration suite asserts the emitted value
  (`internal/integration/publication_test.go:205-225`).
- The integration suite now intentionally holds the store lock while issuing a quote,
  applies suspension, then proves the blocked quote returns `404` and is not persisted
  (`publication_test.go:152-199`). This directly establishes the important
  quote-versus-suspension serialization claim. The added quota/pruning and SQL
  privilege assertions are aligned with the documented bounded-local behavior.

**Final protocol disposition:** accepted for the stated local publication, browse,
selection, and provider-admin slice, subject to the implementation owner's full task
and browser checks completing successfully. This acceptance does not change the
explicit pre-public-release requirement for real anonymous-abuse controls, nor does it
imply payment, order placement, customer authentication, federation, or production
provider administration.
