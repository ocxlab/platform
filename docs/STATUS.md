# Development status

## Restaurant order decisions and cancellation

21 October 2026, Europe/Berlin (UTC+02:00). The next restaurant-operations
checkpoint is implemented and running at http://localhost:18181/. Open a managed
store and select **Order inbox** to inspect and accept or decline guest pickup
orders. Guests can refresh their private receipt and cancel an open, unpaid,
unprepared order, including after acceptance. Provider administration can inspect
a known order through its separately authorized, read-only API.

Agreed terms remain immutable. Operational revisions, attributed events, and
scoped retry results commit together; conflicting actions serialize per order.
The inbox is newest-first with bounded continuation pages. Both UI and external
clients use the same modular OpenAPI operations. New messages and stable errors
have English, German, and Arabic Fluent resources; Arabic still needs a native
speaker's review. See [order operations](ORDER-OPERATIONS.md) for the walkthrough.

Passed: regenerated Go/TypeScript contracts, formatting, full `task check`
(including race tests, real PostgreSQL integration, generation drift, API/web
builds, zero Svelte errors/warnings, and 25 web unit tests), `task smoke`, and all
three Docker browser journeys. Integration covers concurrent actions, tenant,
scope and guest ownership boundaries, event/replay rollback, restart, cursor
continuation, and migration of an actual legacy order while preserving its
original replay. The first browser run found an ambiguous test selector between
two admin forms; exact label matching fixed it and the rerun passed.

Engineering and protocol automated static reviews accepted the bounded slice
after their corrections; these are not independent human reviews. Their final
verification conditions are satisfied by the checks above. Contract regeneration
and formatting also resolved the engineering report's temporary mechanical notes.
The current graph can retain placement, acceptance, and cancellation (three
events); its four-event schema bound must be revisited before more lifecycle work.

No development data was reset. Migration 00004 and the explicit local-only
`task dev:order-access` upgrade retain existing stores and credentials. The local
API was restarted; Vite remains live. `task dev` now stops the old API before
migration to prevent legacy placement writes after the progress backfill.

The original three-part sequence remains: guest placement is complete; restaurant
operations now include manual decisions/cancellation; payment and recovery follow,
then payment-gated kitchen progression. This is not the full restaurant workflow.
Automatic acceptance, opening/pause controls, estimates/timeouts, preparation,
ready/collected states, delivery, notifications, and long-lived guest recovery
remain. No payment or kitchen success is fabricated by acceptance. Network
integration and release validation remain subsequent connected milestones.

## Local Vite development and compact navigation

21 October 2026, Europe/Berlin (UTC+02:00). `task web:dev` now runs Svelte/Vite
locally with pnpm 11.20.0 and the backend/database in Docker. The app keeps
http://localhost:18181/; `task web:container` retains the built Docker alternative.
One imported pnpm lockfile replaces the npm lockfile for both environments.
Dependencies remain pinned; install scripts stay disabled, including esbuild's
postinstall. Container pnpm storage and Svelte's generated files are isolated from
the host's copies, preventing Docker checks from replacing local Vite's paths.

The language control is flag-only at the physical bottom-right, with its named
options opening upward. Header navigation uses Lucide Store and ShieldUser icons,
translated accessible labels, and native tooltips. Both local-Chrome language
tests passed. A temporary source edit updated an already-open page without a
full reload; the probe was removed afterward. Local desktop/mobile rendering
was inspected. The final pnpm Docker browser suite passed all three journeys;
`task web:check` passed (22 unit tests, no Svelte errors/warnings, generated-type
drift check, runtime build). `task smoke` passed. During migration, `task check`
passed backend checks, race tests, SQL integration, and the API build; its web
install was interrupted to fix a slow bind-mounted dependency cache. The web
remainder was rerun successfully with a named Docker cache, including a repeat
install that reused it. A separate Docker Svelte check and another local browser
run verify generated-file isolation. These checks do not certify production readiness.
No development data was reset. Restarting the web process still expires the
existing in-memory development login sessions; reconnect with the same credentials.

## Web languages and bidirectional text

21 October 2026, Europe/Berlin (UTC+02:00). The current web screens now offer
German, English, and Arabic through a persistent header language selector. Each
language name retains its own text direction, including Latin names inside the
Arabic layout. Initial server rendering and navigation set the page direction;
merchant content remains unchanged. See [localization](LOCALIZATION.md).

All three browser journeys passed, covering mixed-direction text, mobile bounds,
no-JavaScript selection, independent browser preferences, and the existing
publication/order/receipt flow. The development app was rebuilt and is running
at http://localhost:18181/. No development data was reset. Arabic translations
still need native-speaker review; these checks are not a full accessibility audit.

At that checkpoint, `task web:dev` started a built Docker image, not a Vite
hot-reload server; the later local-development batch above supersedes this.
Full `task check` passed, including 22 web unit tests and the runtime build.

## Guest order placement and module-owned errors

21 October 2026, Europe/Berlin (UTC+02:00). Batch 1 is complete at the bounded
pickup-placement scope. The earlier pause checkpoint below records its history,
not outstanding work. The contract corrections and additional checks listed there
are now implemented. No development data was reset.

`task check`, the additional `task test`, and `task test:browser` passed. Real
PostgreSQL tests cover concurrent placement with matching and different keys,
price changes versus unavailability, expiry, cross-store/merchant denial,
handler restart, atomic rollback, runtime immutability, tenant RLS, guest-lookup
privileges, and cleanup that preserves order actors. The browser journey now places
and reloads an order, repeats placement without duplication, denies another browser,
and retains private receipt access after public suspension. Engineering and protocol
reviews accepted the corrected bounded implementation; these are automated reviews.

Error codes now have module-owned sources under `contracts/errors/`, generated
Go/TypeScript catalogs, drift checks, and safe fallback tests. Official Fluent
resources provide English and German messages selected per request. The UI maps
known codes to these messages rather than displaying backend details. This is an
error-message foundation, not complete UI translation. See
[the maintenance guide](../contracts/errors/README.md).

Guest access lasts seven days. One active checkout intent is retained per store
in a browser; requesting a new quote replaces it, and an older tab cannot silently
place that superseded intent. Production guest recovery, abuse controls, retention,
taxes and complete restaurant timing remain release work. Acceptance, kitchen
operations and payments follow in the next two authorized batches.

### Earlier pause checkpoint (superseded)

21 October 2026, 19:58 Europe/Berlin (UTC+02:00). Paused at the founder's request.
Batch 1 is NOT complete or deployed; the previous development app remains running.
No development data was reset. Migration 00003 ran only in disposable test databases.

Implemented: store-scoped seven-day opaque guest grants; guest-bound quotes;
pickup placement with immutable quote-derived terms; atomic placement event and
replay; one order per quote; private receipt access. Orders await acceptance, with
payment and preparation not started. Guest secrets are hashed in PostgreSQL and
kept in HttpOnly/SameSite=Strict web cookies, not rendered HTML. Retained cookie
references preserve the quote and original placement key across refresh/retry.
Issued prices survive price-only edits; current selection availability is rechecked.
Extras retain display names, including free choices. Quote expiry is the historical
first-placement deadline, not order expiry. No external effects/workers exist yet.

Passed: Go/TypeScript generation, task fmt, task test (DB suite skips in that
command), task test:integration with race detector, Svelte check (zero diagnostics),
and the existing 13 focused web tests. The new DB journey checks quote ownership,
missing/wrong-audience access, one order per quote, replay conflict, byte-identical
replay after expiry and after withdrawal/quote deletion, private reads and revocation.
The first DB run found a row-lock privilege issue; UPDATE is now granted only on
guest revocation. The rerun passed. No new browser test or full task check ran.

Preliminary engineering/protocol reviews ran. Expired unreferenced guest cleanup
is added; historical actor/replay references remain. Resume with:

1. Fix protocol-review findings: explain historical expiry in FoodOrder.terms using
   a valid OpenAPI 3.0 schema wrapper; distinguish guest vs quote capacity errors;
   remove impossible order 429s; list expected GetGuestFoodQuote errors. Regenerate.
2. Add DB tests for concurrent same/different keys, first-placement expiry, price
   edits vs unavailability, store/merchant isolation, restart/rollback, RLS, guest
   lookup ACL/search_path, runtime immutability, and abandoned-grant cleanup.
3. Test guest cookies and browser placement/receipt/refresh/retry, cross-browser
   denial and secret non-disclosure. Check multi-tab/checkout-intent replacement.
4. Finish engineering/protocol rechecks, task check and task test:browser; then
   task web:dev without reset. Add merchant/provider inspection with the next
   restaurant-operations slice; no inbox, acceptance, payment or kitchen exists yet.

Public abuse controls, production retention/receipt recovery, complete restaurant
terms/taxes/timing, and delivery remain release gates or subsequent slices. The
seven-day guest policy is a bounded foundation choice, not production readiness.

## Lucide action buttons

21 October 2026, Europe/Berlin (UTC+02:00). Added pinned `@lucide/svelte` 1.61.0
to the Svelte 5 application. Shared ActionButton components now serve merchant,
customer, and provider-admin actions. Menu-item deletion uses an icon-only trash
button with the item's name in its accessible label and tooltip. Other actions
retain visible labels alongside their icons; a button without an icon retains
its label as well. Native form submission, disabled states, and handlers remain
unchanged, with no API or storage changes.

Tooltips support pointer hover, keyboard focus, Escape dismissal, and hovering
the tooltip itself. Their position follows the button and stays within the
horizontal viewport; icons are decorative for assistive technology. The shared
component uses a 44-pixel minimum target and a distinct deletion color. Future
dependency additions can use `task web:add PACKAGE=name` inside Docker.

`task check` passed, including 13 focused web tests (five new ActionButton tests),
real PostgreSQL integration checks, generated-contract drift, type checks with
zero warnings, and runtime builds. `task test:browser` passed, exercising deletion
tooltip focus/hover/Escape and horizontal bounds at a 390-pixel viewport alongside
the existing merchant/publication/guest/admin journey. These are targeted checks,
not a comprehensive accessibility audit. `task web:dev` rebuilds the running app
without resetting persisted data; its restart clears development web sessions.

## Managed store table

21 October 2026, Europe/Berlin (UTC+02:00). The merchant workspace now shows
linked store names, menu-item counts, draft revisions, and update times in a
scrollable semantic table. Counts and revisions describe the latest saved draft,
including unpublished changes; variants and extras do not count as menu items.
Before the first save, the table shows zero items, No draft, and store creation
time. Update times use Europe/Berlin and exclude publication/moderation changes.

The authorized store list obtains summaries in one tenant-scoped query; the UI
does not fetch each full menu. The modular ManagedStore schema reuses Store
property definitions and has regenerated Go/TypeScript artifacts. Store summaries
remain available under store:read; full documents still require offering:read.
No database migration or persistent seed changes were required.

`task check`, the regenerated-contract drift check, and `task test:browser` passed.
PostgreSQL checks cover initial summaries, latest private revisions over older
publications, timestamps, and scope separation. The browser checks table headings,
new-store values, two-item revision 1, and one-item revision 2 after deletion.
The development stack is rebuilt with `task web:dev`; restarting the web process
clears development sessions, not stored data or credential validity.

## Managed store navigation

21 October 2026, Europe/Berlin (UTC+02:00). Replaced the merchant workspace's
store-ID entry form with Your stores: named links to the merchant's managed store
workspaces, including stores without published menus. Empty and failed lists have
distinct feedback. Expired/revoked credentials return the user to credential entry.

Added the merchant-scoped `ListStores` OpenAPI operation and regenerated Go and
TypeScript artifacts. It requires `store:read`, retains forced tenant RLS, and
uses bounded cursor pagination (50 default, 100 maximum) over immutable IDs. The
store page resource and pagination parameters have separate reusable source files.
No migration or persistent seed data is needed; existing stores appear immediately.

`task check` passed, including real PostgreSQL list/pagination, empty response,
generated-client, scope/audience, and cross-merchant negative tests. The expanded
`task test:browser` journey passed: an empty merchant creates two stores, sees
both by name, and reopens the first by its link rather than identifier entry.
The development stack is rebuilt with `task web:dev`; its web restart clears
development sessions, but does not reset merchants/stores or their credentials.

## Menu item removal

21 October 2026, Europe/Berlin (UTC+02:00). Added a labelled, non-submitting
Remove item button to each draft menu item. Removal uses the existing complete
revision API; it does not delete retained revisions or immediately change the
published offering. An empty editor explains that saving requires at least one
item and points to Withdraw menu for hiding the whole menu.

`task check` passed, including eight focused web tests. Two new component tests
cover exact-item removal, unchanged source data/base revision, and replacing the
last item. The expanded `task test:browser` journey passed against real API/SQL:
removal survives save/reload, the old item stays public until republishing, and
the remaining item is preserved. `task web:dev` rebuilds/restarts the running web
application without resetting merchant/store data. A web restart clears its
development sessions, so existing credentials may need reconnecting.

## Completed publication and guest-selection slice

21 October 2026, Europe/Berlin (UTC+02:00). See
[PUBLICATION-SLICE.md](PUBLICATION-SLICE.md) for the implemented behavior and limits.
Merchants edit private menus and publish exact revisions; guests browse and obtain
validated ten-minute server-priced quotes. Separate provider-admin credentials
authorize inspection, suspension, and restoration through the supported API and
UI. Atomic audit records retain actor, action, generation, and request correlation.

Contracts use resource-owned source files, shared definitions, explicit access
audiences, and generated Go/TypeScript artifacts. `task check` passed, including
real PostgreSQL race/concurrency/negative checks, six focused web tests, Svelte
checks, drift checks, and both runtime builds. `task test:browser` passed the joined
merchant/guest/admin Chromium journey. Both engineering and protocol reviewers
accepted the bounded local scope after corrections; review reports are under
`reviews/step-3-*`. Public internet abuse controls and production authentication
remain release gates; ordering, payment, and native ONIX integration remain next
connected slices. This is not full F2 acceptance.

`task smoke` and `task web:dev` also passed after the final corrections. The
local web root returned HTTP 200/no-store. The development API/database/web stack
is running at localhost:18180/18181 for a walkthrough; disposable test services
are stopped. Stop with `task stop` without deleting named data/cache volumes.

## Agreed application requirements

21 October 2026: the founder approved SvelteKit/TypeScript web applications and
incremental provider administration alongside the functions being managed. Each
slice assesses its management needs and adds the relevant admin UI and separately
authorized, OpenAPI-defined administration operations in the existing backend.
See [the kickoff plan](IMPLEMENTATION-KICKOFF.md#administration-accompanies-each-feature-slice)
and [applications guide](../apps/README.md). That earlier agreement is now exercised
by the publication slice above; additional administration grows with each feature.

## Completed foundation

21 October 2026, Europe/Berlin (UTC+02:00). Steps 1 and 2 are implemented and
accepted by both technical reviewers at the bounded foundation scope.

Step 1 settled the bounded implementation handoff and received engineering and
protocol reviews. See [FOUNDATION-DECISIONS.md](FOUNDATION-DECISIONS.md) and the
two `step-1-*` reports under `reviews/`. Commit: `c5a710c`.

Step 2 implements authorized create/read store and append/read immutable draft
offering revisions through modular JSON/OpenAPI contracts and actual PostgreSQL.
The replaceable Food model validates a small manually available menu; it does not
publish the catalog or validate customer purchase selections. Generated Go models,
client, routing, and embedded contract are committed alongside their modular sources.

Executed successfully: `task doctor`, `task contracts:generate`, `task check`, and
`task smoke`. The check includes contract/example validation, deterministic bundling,
non-mutating generated drift comparison, vet, race-enabled tests, isolated real
PostgreSQL integration, and the non-root API image build. Smoke applies development
role/migrations explicitly, starts API/database, checks HTTP liveness, rejects an
unissued credential through the actual database-backed lookup, and checks SQL
connectivity. It creates no merchant or store fixtures in the development database.

The integration suite migrates a unique disposable database twice and uses the
non-owner `ocp_app` role. It exercises a generated-client store request, read/write
contracts, invalid/expired/revoked/wrong-action/wrong-merchant credentials, forced
row-level security, pooled tenant-context cleanup after commit/rollback, concurrent
revision and retry-key writers, atomic rollback, immutable historical drafts, and
handler restart against retained data. Unit-only runs explicitly skip DB integration;
the dedicated Task supplies a disposable PostgreSQL service and stops it afterward.

No production authentication, storefront, publication, ordering, inventory allocation,
payment, native ONIX runtime, UCP interoperability, merchant transfer, cloud failover,
or storage-loss recovery was implemented or tested in this slice. `services/onix/`
remains a planned integration boundary, not a substitute local discovery service.
No CI, cloud account, real credentials, or research source snapshots were added.

The final `task check` and `task smoke` reruns passed after the review corrections;
local Markdown links passed a filesystem check. `task stop` completed afterward;
development containers/network are stopped/removed and named development data/cache
volumes are retained. Resume with `task smoke` or `task dev`; no host language installation is needed.

Both step-2 technical reviews and their verification/disposition are retained under
`reviews/`. All reported blockers and bounded hardening/maintainability findings
were corrected. The engineering reviewer also executed contract checks, real
PostgreSQL integration, race tests, and the runtime build. These are automated
technical reviews, not independent human or interoperability certification.

Read [FOUNDATION-REPORT.md](FOUNDATION-REPORT.md) for the result and acceptance
boundary. Next: agree the connected restaurant publication/purchase slice under
the kickoff plan, retaining network and payment integration milestones. No next
slice has been implemented as part of this batch.
