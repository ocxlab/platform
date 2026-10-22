# Internal Shopping and Lodging cohort — engineering review

Reviewed 2026-10-22. This is a static review of the uncommitted internal-domain
slice. It did not run Docker, PostgreSQL, race checks, or the optional pinned
upstream schema fetch. Parent-reported earlier PostgreSQL/race results therefore
are not independently reproduced here.

## Scope

The slice adds a separate internal HTTP listener for controlled Shopping and
Lodging session cohorts. Food's public router and model registry remain separate.
It shares tenant transactions, replay mechanics, and payment terms, while Shopping
and Lodging own their catalog, allocation, session, and terms tables. The selected
payment instrument records an outstanding `pay_later` obligation; it does not
represent a successful charge.

## Corrections rechecked

### Closed: security definer grant lookup has a bounded owner and result shape

`commerce_internal.lookup_grant` in migration `00018_internal_domains.sql` is a
`SECURITY DEFINER` function, but does not set an explicit non-runtime owner or
its owner privileges. It also returns the complete `commerce_internal.grants`
row, including the credential digest, rather than the small principal/profile
projection used by the handler. The new schemas are not covered by the existing
runtime-role safety checks.

The corrected migration creates `ocp_internal_auth_owner` as a no-login,
non-superuser, non-bypass-RLS owner, grants it only the required table access,
and transfers ownership of the fixed-search-path function. The function now
returns only the principal/profile/management fields needed by authentication.
The supplied integration assertions inspect the owner, ACL, path, lack of
application membership, and lack of direct grant-table reads. `CheckRuntimeRole`
also covers the new schemas and definer ownership.

### Closed: profile admission is server-owned

`ucpinternal.Provision` accepts any structurally-valid `Profile`. The current
validation permits arbitrary service/payment-handler entries and need not contain
the capability for the credential's bound domain. That does not implement the
stated server-owned static profile allowlist.

`allowedProfile` now accepts only exact controlled Shopping, Lodging, or
combined controlled profiles and requires the credential-bound capability. The
same check applies on provisioning and authentication. The profile URI is now
also the exact `ControlledProfileURI`, not merely a structurally valid HTTPS
location. The admission regression rejects an arbitrary URI, service, and
payment handler. This preserves the no-runtime-fetch boundary.

### Closed: Shopping order read is treated as a read-only permalink target

Shopping completion constructs `permalink_url` as
`{internal endpoint}/shopping/orders/{id}`, but the internal OpenAPI contract and
router provide no matching order-read operation. A completed response therefore
contains a URI that the listener cannot serve.

An order route now queries immutable terms under the same merchant, store, and
customer binding. `ucpinternal.execute` consistently treats `order` as a
read-only action for body parsing and replay persistence, and the focused test
covers success and a wrong-customer denial. Add a GET-without-`Content-Type`
regression as a small compatibility guard; the implementation does not currently
depend on that header for this action.

## Sound bounded choices observed statically

- The public listener has no Shopping/Lodging route registration. The internal
  listener uses a distinct `internal` role and per-domain route admission.
- Credentials are opaque and only their digests are stored. Authentication binds
  merchant, store, domain, customer, and profile before a domain operation runs.
- Shopping rechecks the authoritative catalog revision, locks selected stock rows
  in deterministic order, and decrements them in the same completion transaction.
- Lodging uses ordered unit advisory locks and a `[check_in, check_out)` exclusion
  constraint, so application and database constraints both prevent overlapping
  confirmed allocations. Expired sessions allocate neither stock nor rooms.
- Terms, the domain completion record, and the pay-later obligation are committed
  together. No production payment or charge completion is claimed.

## Acceptance boundary

Retain the targeted PostgreSQL/race coverage for credential denial, replay after
restart, cross-store/domain/customer
denial, oversold Shopping completion, Lodging overlap/adjacency, expiry without
allocation, and public-listener route absence. Parent-reported targeted process
tests are not independently rerun by this reviewer. The result would remain an
internal controlled cohort—not public UCP conformance, a public Shopping/Lodging
offering, generalized discovery, or payment-provider integration.
