# Publication and guest selection slice

21 October 2026, Europe/Berlin (UTC+02:00). Bounded implementation report for the
publication, guest-selection, and incremental administration slice.

## Bounded outcome

Merchants can edit private drafts and publish an exact revision. Guests can browse
the published menu and request a server-priced selection quote. Provider founders
can inspect and suspend publication through separate administration permissions.
SvelteKit/TypeScript supplies the merchant, storefront, and administration surfaces
in one initially compact web deployment, with shared components and generated
OpenAPI types. This does not select production hosting.

## Decisions

- Publication has a generation independent of the immutable draft revision. Publish,
  withdrawal, and administrative suspension serialize on the same store and require
  the expected generation. A stale writer gets a conflict, not silent overwriting.
- Suspension preserves the merchant's chosen revision. Restoring publication
  visibility never publishes an unreviewed draft. Each change records actor,
  action, reason where applicable, resulting generation, and request correlation
  atomically in an append-only audit record.
- Provider administration uses separate credentials and action scopes, not a
  merchant grant carrying a universal tenant bypass. Credentials remain a local
  development bootstrap, not production identity management.
- Public projection contains only explicitly published store/menu information.
  Drafts, grants, audit records, and unpublished histories remain private.
- Food selection is a separate domain operation over the published model. The
  server checks item availability, variant and option references, duplicate choices,
  cardinalities, bounded quantities, and integer prices. It retains immutable
  ten-minute quote snapshots. A quote is not an order, acceptance, reservation,
  kitchen instruction, or evidence of payment. Placement must later implement an
  explicit policy for quote expiry and changed availability/publication.
- OpenAPI source stays modular by resource and API audience, with shared identifiers,
  errors, headers, and model definitions. Types and routing are generated; domain
  behavior and authorization remain explicit code.
- Web session plumbing must not expose bearer credentials in page data, browser
  storage, or logs. Business decisions stay behind the same supported APIs used by
  non-UI clients. UI sessions are development scope until production auth is added.

## Executed acceptance

`task contracts:generate`, `task web:generate`, `task fmt`, and full `task check`
passed. The latter includes generated Go/TypeScript drift checks, source/example
validation, Go vet/race tests, actual PostgreSQL integration, six focused web
tests, Svelte diagnostics (zero errors/warnings), and both runtime image builds.
The PostgreSQL suite actually ran with its test database; a unit-only skipped
database suite is not the reported acceptance.

`task smoke` passed after applying the development migrations without resetting
application data. `task web:dev` built and started the local stack; an actual host
HTTP request to localhost:18181 returned 200 with `Cache-Control: no-store`.
The development stack is left running for a walkthrough. Disposable browser/test
services are stopped, and named development data/cache volumes are retained.

`task test:browser` passed its joined Chromium journey against the actual Go API
and disposable PostgreSQL: create a store, edit/save a private menu with required
extras, verify draft invisibility, publish, reject an incomplete selection, quote
two meals at the server-calculated EUR 15.00 total, suspend visibility as admin,
and restore the same publication. It also checks separate browser sessions,
credential absence from rendered HTML/localStorage, cookie attributes, and
cross-origin mutation rejection. No screenshot/trace containing tokens is retained.

Backend integration checks additionally cover audience/action/tenant denials,
revoked credentials on replay, exact mutation replay/conflicts, stale generations,
concurrent publication writers, quote blocking behind an actual SQL publication
lock and rejection after suspension, draft/publication separation, immutable
retained prices, option/availability/quantity validation, capacity/expiry cleanup,
audit correlation, and restricted-role quote/audit/credential-table privileges.

Engineering and protocol reviewers accepted the bounded implementation after
corrections. Their reports are in `docs/reviews/step-3-engineering.md` and
`docs/reviews/step-3-protocol.md`. These are automated reviews, not independent
human review, security certification, or interoperability certification.

## Limits and next slice

The quote operation is deliberately non-idempotent: retries create fresh quotes
but no commercial effects. At most 1,000 active quotes per store bound local
storage; capacity returns 429 and `Retry-After: 60`. Creation prunes up to 1,000
expired quotes for that store. Quiet stores can retain expired snapshots until
the next successful creation; expiry is not a deletion guarantee. This cap is
not adequate public abuse control: an attacker could fill a store's active slots.
Rate limits and retention need explicit pre-release implementation and tests.

The web bootstrap keeps bearer credentials in server memory behind one-hour
audience-bound opaque cookies. Restart signs users out. It is not production
login, distributed session storage, signup, or delegated authorization. Public
storefronts are currently opened by their known link; no discovery index exists.
Food selection supports EUR/manual availability and variants/extras, not delivery
serviceability, tax/legal completeness, quantitative stock, or generic domains.

Next connect expiring quotes to authorized placement, immutable purchase terms,
merchant acceptance and preparation signals, and durable work. A quote ID alone
must not grant authority to place or inspect a purchase. Add order-related admin
inspection when its owning behavior is implemented, not as speculative endpoints.

Orders, payment adapters, delivery serviceability, native ONIX publication, UCP
compatibility, production authentication, and cloud deployment are not acceptance
claims of this slice. Public internet abuse controls require explicit validation
before launch; development exposure remains loopback-only.
