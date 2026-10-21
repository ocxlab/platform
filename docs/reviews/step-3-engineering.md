# Step 3 engineering review — pre-implementation guardrails

Reviewed 21 October 2026, Europe/Berlin. This is an early design review of the
bounded publication, public browse, Food selection/quote, and incremental web UI
slice. It is not a review of implemented code and does not make any acceptance
claim.

## Bounded verdict

**Proceed with the proposed slice.** It is a useful next increment because it
makes a merchant's retained Food draft selectively visible and lets a guest get
immutable, time-bounded terms without silently creating an order, allocating
stock, taking payment, or claiming network publication. The proposed separate
publication pointer/generation is simpler and safer than editing a draft into a
“published” state.

Keep the implementation compact: one provider runtime and PostgreSQL transaction
boundary are sufficient. Do not add a message broker, ONIX runtime, UCP checkout,
payment adapter, customer account system, stock allocator, or a generic product
engine merely to complete this slice.

## Required implementation boundaries

| Priority | Boundary or risk | Minimum safe implementation and evidence |
| --- | --- | --- |
| Required | Draft privacy | Public GET queries must join through the current publication pointer and return a purpose-built public projection only. Never deserialize/return a raw offering revision or use a merchant-scoped draft read as a public handler. Test unpublished, withdrawn, suspended, other-merchant, and guessed-revision paths returning no draft data. |
| Required | Publication state | Keep immutable offering revisions separate from a per-store publication record containing at least referenced revision, monotonically increasing generation, visible/suspended state, and timestamps. Publish, withdraw, and provider suspend must lock the same store/publication row, compare an expected generation, write the audit event and idempotency result in the same transaction, and never mutate a retained revision. Test concurrent publish/withdraw/suspend and same-key replay. |
| Required | Provider authority | Provider credentials must use a distinct hashed-token table and narrowly scoped lookup, never a merchant grant with a magic merchant ID. Provider administration operations require their own declared API scopes and emit an attributable actor type/id in an append-only audit record. The browser UI must not receive provider tokens. |
| Required | Quote is not an order | A guest selection is evaluated server-side against the currently published immutable revision while a transaction holds the relevant store/publication row lock. The retained quote must contain the publication generation, offering model/version/revision, selected lines, currency, integer minor-unit totals, issued/expiry times, and an opaque quote ID. It must say neither “reserved” nor “accepted”; it creates no order, capacity allocation, payment operation, or merchant notification. Test expiry and withdrawal/republication behavior explicitly. |
| Required | Food arithmetic and selection | Ignore any client-supplied price or availability conclusion. Validate item/variant/option identifiers, option-group cardinalities, manual availability, bounded quantity/counts, currency consistency, and checked integer arithmetic. Reject overflow and invalid/missing required options before quote persistence. Use integers/minor units only. |
| Required | Browser session boundary | SvelteKit server routes may use a short-lived opaque local session cookie, but the browser must never store or receive backend bearer grants. Mark cookies `HttpOnly`, `SameSite=Lax` (or stricter), scoped narrowly, and `Secure` outside explicitly documented local HTTP development. Protect state-changing server routes against cross-site requests through origin validation or a CSRF token. UI calls should go through server-side route handlers using the generated TypeScript client. |
| Required | Contract/API consistency | Extend the modular OpenAPI source before handlers. Keep public browse/quote, merchant publication, and provider administration paths in distinct owned path/schema areas. Generated TypeScript and Go clients must come from the same bundled source. Mark protected actions with `x-required-scope`; health and public paths explicitly opt out of bearer security. |
| Required | Audit semantics | The audit record should identify the actor, action, merchant/store target, prior/new publication generation or suspension state, request/replay identifier, and timestamp. It records an administrative decision, not a customer order/payment event. Runtime privileges must allow append/read only as required; no generic audit-table update/delete grant. |

## Deliberate simplifications

- A public projection may be assembled at read time from a published revision;
  a separate search index/cache is not required yet. Its future introduction must
  preserve the same visibility rule and generation semantics.
- One provider administrator role is enough for this slice. Fine-grained provider
  staff roles and impersonation are deferred.
- A quote may be guest-accessible without a customer account. Do not add saved
  addresses, customer profiles, checkout, payment, or consent flows.
- Server-rendered SvelteKit routes can be the initial BFF/session boundary. A
  separate authentication service or frontend microservice is not needed.
- Provider-custom domains, `/mcp`, and custom storefront hosting remain
  addressing work, not prerequisites for a local browse/quote test.

## Acceptance evidence for the later review

The final review should require actual PostgreSQL/API/UI evidence for:

1. Merchant publish/withdraw with correct generation and replay behavior.
2. Provider-admin suspend/restore denial paths and immutable audit records.
3. Public browse returns only allowlisted published fields, never drafts or
   administration data.
4. Valid Food selection produces a retained ten-minute quote; unavailable,
   malformed, stale, overflow, and expired selections do not.
5. Concurrent publication/quote boundaries do not attach a quote to an
   unverified or changed publication generation.
6. SvelteKit merchant, storefront, and provider-admin happy and denied paths use
   the generated TypeScript client without bearer tokens in browser storage.
7. Docker/Task runs the API, PostgreSQL, frontend checks, and Playwright against
   the real local runtime; no claim is made for ONIX/UCP/orders/payments.

These are sufficient for the slice. The later technical review should look for
actual behavior and authority boundaries, not demand unfinished federation or
fulfilment features.

## Static implementation review — 21 October 2026

Reviewed the uncommitted implementation of the bounded publication, public
browse, Food quote, and SvelteKit UI slice. This pass intentionally did not run
the shared PostgreSQL or browser tasks while their owner had an active runner.

### Findings

| Severity | Finding | Exact location | Required bounded correction |
| --- | --- | --- | --- |
| Medium | The public quote operation says that authenticated mutation retries return the original body. It is explicitly anonymous and deliberately non-idempotent; the implementation and integration test correctly create a distinct retained quote for a repeated request. Generated TypeScript therefore also teaches the wrong client behavior. | `contracts/openapi/v1/paths/food-quotes.yaml:20`; generated from it in `apps/web/src/lib/api/generated.d.ts` | Describe the actual contract: every successful request creates a distinct immutable quote and has no authenticated/idempotency-key replay behavior. Regenerate the committed clients after the source correction. |

### Reviewed boundaries

- `commerce.stores` now holds a separate immutable-publication pointer,
  projection, generation, and provider suspension state. Publication and quote
  paths lock the same store row, so a quote cannot survive a concurrent
  suspension or revision change with an unverified generation.
- The public handler joins the current, non-suspended publication and returns a
  model-owned projection rather than a raw draft. `food.Project` enumerates the
  Food model through a concrete struct, and the integration coverage checks
  unpublished, suspended, withdrawn, and draft-update behavior.
- Food pricing uses published fields only and checks identifiers, group
  cardinalities, manual availability, bounds, and minor-unit overflow. The
  resulting ten-minute snapshot is not an order, reservation, payment, or
  acceptance.
- Merchant and provider credentials have separate tables, token formats, lookup
  functions, scopes, and replay tables. Provider actions carry an attributable
  actor and request UUID in the same transaction as the publication change and
  append-only audit write.
- The local SvelteKit bridge retains opaque, audience-specific `HttpOnly`,
  `SameSite=Strict` cookies server-side and passes bearer values only from server
  actions to the generated OpenAPI client. It is correctly described as a
  development bootstrap rather than production authentication.
- Quote capacity and expired-snapshot pruning are bounded. This is storage
  protection, not a substitute for public-internet rate limiting, which remains
  a later operational control and is not falsely claimed here.

### Bounded verdict

**Conditional acceptance after the medium contract correction and regenerated
artifacts.** I found no further implementation blocker in the reviewed scope.
Final verification should run the declared Docker/Task checks once the active
browser runner is finished, and should include the existing PostgreSQL tests for
the provider lookup boundary, publication audit correlation, quote locking, and
quote quota/pruning.

## Static disposition update — 21 October 2026

The conditional finding is resolved.

- `food-quotes.yaml` now correctly states that a repeated anonymous quote
  request creates another immutable quote without commercial effects; the
  regenerated TypeScript declaration carries the matching operation
  description.
- The bounded quote-capacity response now declares `Retry-After`, the handler
  supplies `60`, and the PostgreSQL integration test asserts it alongside
  capacity and expired-snapshot cleanup.
- The integration suite contains a real store-row lock wait where a public
  quote is held behind a concurrent suspension and returns `404` without a
  persisted quote after the suspension commits.
- The local opaque-session map now records and verifies its credential
  audience; focused tests cover cookie opacity/protection, copied-handle
  rejection across merchant/provider audiences, and expiry.

**Static acceptance:** no remaining engineering blocker is identified for this
bounded slice. This is not execution acceptance: retain the pending full
`task check` result as the final runtime/build verification record.
