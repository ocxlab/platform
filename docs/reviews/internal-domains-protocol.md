# Internal Shopping and Lodging cohort — protocol review

Reviewed 2026-10-22, Europe/Berlin, against the uncommitted implementation and
the pinned UCP source snapshot
`d13cc61455d2a90f7e6038df07fdbe72dcccdcc7`. This was a static protocol and
security review. I did not run Docker, PostgreSQL, race, browser, or network
tests. Parent-reported successful PostgreSQL/race journeys and upstream-schema
response validation are therefore useful implementation evidence, not checks I
independently executed.

## Scope assessed

The change adds an isolated internal listener with the five UCP-shaped Shopping
checkout and Lodging booking session operations, a credential-bound profile,
and a deliberately narrow `pay_later` instrument. It does not mount these
routes on the Food/public listener, claim public UCP conformance, expose a
general profile/schema fetcher, or report a successful payment.

The session adapters make the right bounded distinctions: a Shopping completion
locks and decrements authoritative stock; a Lodging completion both serializes
units and has a database overlap constraint; and both commit immutable domain
terms, the completed record, and an `outstanding` financial obligation in the
same tenant transaction. `pay_later` is not represented as a charge,
authorization, or settlement result.

## Required corrections before bounded acceptance

### 1. Constrain the controlled caller profile at provisioning

`services/provider/internal/ucpinternal/auth.go` accepts any structurally valid
profile URI in `Provision`, while the runtime only compares the request header
with that per-grant value. This is not yet the stated static allowlist fixture:
a bootstrap caller can provision an arbitrary HTTPS `/.well-known/ucp` URI,
and `validProfile` does not require the credential's own domain capability.

Use a named, server-owned profile fixture/registry and require exact equality
with the selected fixture for the grant's domain. Require that the selected
profile includes the corresponding `dev.ucp.shopping.checkout` or
`dev.ucp.lodging.booking` capability. Keep the current no-fetch property; do
not add a dynamic URL fetcher. Add a negative provisioning test. This matches
the internal-bearer fixture boundary in
`ocp-lab/docs/system-design/reviews/THREE-DOMAIN-WIRE-CHECKLIST.md`.

### 2. Make the advertised pay-later schema URI retrievable from this listener

`sessionkit.Meta` and `ucpinternal.ControlledProfile` declare
`https://ocplab.org/schemas/internal/pay-later.json`, whereas the implemented
route is served as
`{OCP_INTERNAL_ENDPOINT}/schemas/internal/pay-later.json` (the fixture uses
`https://internal.ocplab.org`). Consequently a client following the declared
payment-handler schema URI does not reach the actual controlled schema.

Derive the advertised URI from the validated internal endpoint, or serve the
declared `https://ocplab.org` URI consistently. Use the same URI in the profile
and each session's `ucp.payment_handlers` metadata. Add a profile assertion
that fetches/validates the exact advertised schema route under the controlled
fixture. This is especially important because the completed-request payment
object identifies the pay-later handler.

### 3. Apply the already identified narrow `SECURITY DEFINER` correction

Migration `00018_internal_domains.sql` gives `ocp_app` a
`SECURITY DEFINER` lookup that returns the whole `commerce_internal.grants` row,
including the credential hash, without establishing the intended restricted
function-owner model. The runtime should receive only the grant principal,
stored profile, and management flag it consumes. Use an explicit no-login,
non-bypass-RLS owner with the minimum table rights, fixed search path, revoked
`PUBLIC` execute, and an `ocp_app`-only grant. Extend the runtime-role and
migration assertions to these new schemas/functions.

### 4. Remove or implement the Shopping confirmation link

`shopping.Service` constructs
`{internal endpoint}/shopping/orders/{id}` as `order.permalink_url`, but neither
the internal contract nor the router offers that resource. A completed session
therefore returns a syntactically valid but dead URI. Either add a scoped
credential-bound order read operation, point to an existing usable owned order
resource, or omit the optional link if the selected upstream schema permits
that. Do not leave a confirmation URI that a caller cannot use.

## Protocol observations retained for implementation

- The internal contract correctly uses upstream `Request-Id` and UUID
  `Idempotency-Key`, not the owned API's `X-Request-ID`; replay is scoped to the
  credential, domain, operation, resource, and canonical request body.
- `UCP-Agent` is syntactically restricted to an HTTPS
  `/.well-known/ucp` URI and checked against the credential. It remains profile
  metadata, not customer/session authority: the bearer-derived customer/store
  scope is used for session reads and mutations.
- Both upstream root schemas require `payment` at `complete`. The explicit
  selected instrument plus an `outstanding` obligation is a sound bounded way
  to meet that structural requirement without claiming a PSP payment outcome.
  Its handler/instrument schema and profile advertisement must remain aligned.
- Returning a completed session for a later *new* completion key does not repeat
  stock allocation or create a second obligation; the implementation's terminal
  session load protects that result. Matching replay keys additionally preserve
  the stored response. Continue to test both paths after fixes.
- The `PUT /{domain}/catalog` operations are owned internal catalog-management
  surfaces, not part of the selected upstream five-session operation target.
  Keep that distinction visible in API descriptions and never use their presence
  as UCP catalog compatibility evidence.

## Re-review and disposition — 22 October 2026

This re-review is static. I did not run the parent-reported six PostgreSQL/race
tests, generated-client/profile/schema retrieval, order-isolation checks, or
upstream/owned response validation.

Three corrections now verify in source:

1. `commerce_internal.lookup_grant` returns only the consumed principal/profile
   projection; migration `00018` assigns it to the no-login,
   non-superuser/non-bypass-RLS `ocp_internal_auth_owner`, fixes its
   `pg_catalog` search path, revokes `PUBLIC`, and grants execute only to
   `ocp_app`. `CheckRuntimeRole` now also covers the new schemas and definer
   owner. The integration assertions inspect the owner, ACL, result type, and
   runtime inability to read token hashes.
2. The business profile returned by
   `services/provider/internal/ucpinternal/server.go` derives its pay-later
   schema/spec URI from the configured internal endpoint. Session metadata now
   identifies the negotiated handler instance without inventing a second static
   schema host. The contract and test assert the advertised internal route.
3. `GET /shopping/orders/{id}` is now an explicitly owned, credential-bound
   read contract and is labelled as **not** an upstream UCP Order API. Its
   customer/store predicate backs the checkout confirmation permalink; the
   integration test includes a cross-customer 404.

### Closed: static caller-profile URI admission

`ucpinternal.ControlledProfileURI` is now the exact server-owned fixture URI,
and both `Provision` and `authenticate` require equality with it. The negative
integration fixture attempts a second syntactically valid HTTPS
`/.well-known/ucp` URI and expects provisioning to fail. Together with exact
controlled-profile document comparison and the domain-capability check, this
closes the static caller-profile admission finding. No runtime profile fetch is
introduced.

## Bounded verdict

**Static acceptance at the bounded cohort scope.** The four original corrections
and the profile-URI follow-up now verify in source. Parent-reported PostgreSQL,
race, generated-client, and response-validation checks remain parent-owned
execution evidence, not tests independently performed by this review. The
result remains a pinned internal UCP-shaped interoperability experiment, not
UCP conformance, a public Shopping/Lodging offering, generalized discovery, or
payment-provider integration.

## Source locators

- UCP pin `d13cc61455d2a90f7e6038df07fdbe72dcccdcc7`:
  `source/services/shopping/rest.openapi.json#/components/parameters/{request_id,idempotency_key,ucp_agent}`;
  equivalent Lodging locators; and
  `source/schemas/{shopping/checkout,lodging/booking}.json#/properties/payment`
  (`ucp_request.complete: required`).
- [UCP implementation wire checklist](../../../ocp-lab/docs/system-design/reviews/THREE-DOMAIN-WIRE-CHECKLIST.md)
