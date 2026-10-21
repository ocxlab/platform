# Development status

## Readable merchant workspaces and aligned administration controls

22 October 2026, Europe/Berlin (UTC+02:00). Open stores from the merchant list
to use readable workspace URLs such as
`/merchant/stores/doener-karlsruhe-store-1`. The name makes the URL recognizable;
the persistent provider-local handle disambiguates stores. Each page and mutation
resolves that handle through the authenticated, merchant-scoped list API's exact
`handle` filter. Unknown and foreign handles return no record. The UI does not
scan every store or read the database directly. UUIDs still identify API resources.
As requested for this pre-launch app, the previous UUID workspace routes are
removed rather than retained as compatibility routes. Customer storefront and
receipt URLs, custom domains, and subdomains are separate work.

Menu and Orders tabs now share a store heading and a route back to Your stores.
They remain visible on order details and identify the current area. The dangling
store-handle paragraph is removed. Administration now separates aligned filter
fields from an aligned Search/Clear filters action row. Clear filters and Refresh
status have secondary-button styling while retaining navigation-link semantics.
English, German, and Arabic labels are included, with logical-direction styling.

`task check` passed, including real PostgreSQL isolation/lookup cases, generated
Go/TypeScript drift checks, 56 web unit tests, and zero Svelte diagnostics.
`task smoke` passed and updated the local API without resetting development data.
All four built-app browser regressions passed, including tab navigation, readable
workspace links, toolbar geometry, narrow-screen/RTL layout, no-JavaScript admin
navigation, and the existing ordering/development-payment/pickup journey. Desktop merchant
and administration screenshots and the mobile administration screenshot were
visually inspected. An initial browser assertion read the URL before navigation
completed; it now waits for the expected route. No application workaround was
needed for that test-timing issue. All four Vite browser regressions also passed
after adding explicit completed-state assertions before cross-browser refreshes
and subsequent navigation. The initial Vite run refreshed the customer before
merchant acceptance finished; the tests now wait for saved acceptance, payment,
and kitchen progress rather than relying on relative browser speed. Mutation
tools were not rerun for this navigation/layout batch.
Automatic status updates, order-history styling, and customer URL changes remain
in [the interface backlog](UI-UX-NEXT.md); this batch does not implement them.

## Browse administration and make menu extras discoverable

22 October 2026, Europe/Berlin (UTC+02:00). Provider administration now has
Stores and Orders navigation, searchable/status-filtered tables, configurable
page sizes, forward/backward navigation, and row actions. Inspecting a record
no longer requires copying UUIDs. Orders remain read-only in administration;
visibility changes retain their reason, generation, and idempotency protections.
The new OpenAPI list operations expose bounded summaries under the separate
provider credential audience. Migration 9 uses narrowly scoped read-only SQL
functions without weakening tenant RLS. See [contracts](../contracts/README.md#provider-administration-browsing).

The menu editor now separates sizes/variants from extras/additions, explicitly
labels creation actions, and offers visible hints plus an expandable explanation.
Extra cheese can add EUR 1 while garlic adds EUR 0; free prices are also explicit
on the storefront. New extras groups are optional by default. Selection limits
remain available under Selection rules. Tooltips have larger text and no longer
repeat visible button labels. Important guidance does not depend on tooltips.
English, German, and Arabic are included. [Interface conventions](UI-UX.md)
record these patterns for subsequent screens; this is not a complete UI redesign.

The existing engineering and protocol reviewers completed automated static
reviews. Their requested SQL-function ACL assertions and explicit error-response
contracts are implemented; see [engineering](reviews/admin-browse-engineering.md)
and [protocol](reviews/admin-browse-protocol.md). These reviews are not independent
human usability or security certification.

`task check` passed: generated-contract drift, Go formatting/vet/race checks,
PostgreSQL integration, API/web builds, 48 web tests, and zero Svelte diagnostics.
All four built-app and all four Vite Chromium journeys passed. Coverage includes
unfiltered and filtered admin tables, forward/backward pages, preserved filters
after inspection, read-only orders, RTL table alignment, narrow-screen containment,
no-JavaScript admin navigation, and a pizza with paid cheese and free garlic.
The wider publication/checkout/pickup regression captures page errors and hydration
warnings; none were captured on its final run. Screenshot inspection covered the
desktop extras editor/admin tables and mobile order list. The in-app browser
connection was unavailable, so verification used the project's Docker Playwright
runner. This is not an accessibility certification. Mutation tools were not rerun
for this UX batch; prior scores do not measure these new components or SQL.

Regression testing exposed an empty optional search parameter being rejected by
the API. Unfiltered web reads now omit it, and both initial tabs have regression
assertions. The actual local database migrated to version 9 without resetting
development records, and `task smoke` passed. Local Vite still serves port 18181.
No real processor calls or money are involved. Production search-load testing,
rate limiting, real payment adapters, and joined-network integration remain work.
The proposed main restaurant batch after this UX detour is ordering controls
(pause/resume and manual/automatic acceptance), not another admin redesign.
The user's subsequent UX feedback is queued in [UI follow-ups](UI-UX-NEXT.md):
merchant tabs, toolbar alignment/action styling, automatic status updates, order
history presentation, and a primary-source restaurant-tool workflow comparison.

## Separate checkout and merchant pickup progress

22 October 2026, Europe/Berlin (UTC+02:00). Public store cards now show the
business name without a dangling internal handle (`838e542`). The local simulator
uses **Pay order → development checkout → receipt**. Returning without choosing
an outcome does not pay; customers can resume checkout. This remains a first-party
simulation, not an embedded Stripe/Mollie implementation. See the updated
[walkthrough and real-provider boundary](LOCAL-PAYMENTS-AND-REFERENCES.md).
The checkout, hydration hardening, and pickup slice are committed as `7a779cd`.

Merchants can now start preparation, mark readiness, and record customer pickup
for accepted, paid, open pickup orders. The same authorized, modular OpenAPI
operation serves the UI and API clients. Progress, attributable history, revisions,
and idempotent responses commit together. Cancellation races against preparation
under the same order lock; preparation stops automatic cancellation. These are
merchant reports, not independent proof of physical work. Cash payment, own
delivery, and real-processor adapters remain separate slices.

Vite regression testing exposed pre-hydration interaction races that the faster
built app did not reveal. JavaScript-only editor buttons now wait for mounting;
the storefront submits native named controls instead of hydration-dependent hidden
JSON, and its initial effect no longer resets customer selections. Lucide is
excluded from dependency prebundling to keep it in the Svelte compiler pipeline.
The exact original `undefined.call` stack was not reproduced and is not claimed
as an established root cause. Browser tests capture hydration warnings and page
exceptions rather than relying only on a root hydration marker.

The development-mode run also found SvelteKit's built-in form-origin check is
skipped in Vite. An explicit web hook now applies the same unsafe-form origin
boundary locally; the real local Vite probe rejects a foreign-origin form with
403 while allowing same-origin forms to reach validation. No production check
was disabled. Browser failure diagnostics persist in ignored `apps/web/test-results/`.

The full `task check` passed after the native-form projection follow-up: generated
contract drift, Go formatting/vet/race tests, PostgreSQL integration, API/web
builds, 40 web tests, and zero Svelte errors/warnings. After the final native-control
and origin-guard refinements, `task web:check` passed with **43 web tests**, clean
generated drift, zero diagnostics, and a successful build. All three final built-app
and all three Vite browser journeys passed, including no-JavaScript initial-item selection/quote, checkout
return/resume/decline/retry/payment/refund, and preparation through pickup.
`task smoke` also passed. No hydration warnings or page exceptions were captured
in the final merchant/customer browser workflow. Chrome
opened the preserved local Döner Karlsruhe storefront without captured
browser exceptions or hydration warnings. The actual API/database migrated to
version 8 without resetting development records; local Vite remains on port 18181.

Both engineering and protocol static review reports are in
`docs/reviews/checkout-pickup-*.md`. Their requested neutral event-helper ownership,
same-revision cancellation/preparation race, and retained pickup-action replays
are implemented. Stryker and Gremlins were rerun; their bounded results and
remaining survivors are recorded in [mutation testing](MUTATION-TESTING.md).
No real payment credentials, processor calls, or money were used in this batch.

## Reliable refresh, readable references, and local payments

21 October 2026, Europe/Berlin (UTC+02:00). Explicit status refresh now reloads
the current receipt/order/inbox from the server, including repeated clicks on the
same URL. The fix is committed as `e904d1f`; subsequent work below builds on it.

Orders now display provider-unique, non-resetting references such as `O-000042`;
stores receive stable handles such as `store-1`, searchable in the public directory.
UUIDs remain identity and authorization keys. Migrations preserve records and
historical idempotency responses. The running development store Döner Karlsruhe
was retained and now displays `store-1`.

The local Payment module persists simulated checkout, declines, abandonment,
success, and cancellation/refund outcomes with the order's operational history.
It is explicitly enabled in local API/web tasks and disabled by default in the
handler. No real money or processor credentials are used. It does not simulate
kitchen progress or establish real payment-processor compatibility. See the
[walkthrough](LOCAL-PAYMENTS-AND-REFERENCES.md).

Executed verification:

- `task check`: modular contract validation/drift, formatting, Go vet/race tests,
  PostgreSQL integration, Go/web builds, Svelte diagnostics (zero), and 35 web tests.
- `task test:browser`: all three Chromium journeys passed, including customer and
  merchant repeated refresh, simulated decline/retry/success/refund, catalog
  publication/deletion, provider visibility, and English/Arabic UI regressions.
- `task smoke`: preserved and migrated the local database and checked API liveness,
  rejected credentials, and SQL connectivity. It is not a commerce readiness check.
- `task test:mutation:web`: 159 of 160 mutants killed, one survivor, no uncovered
  mutants/timeouts/errors (99.38%) in four selected TypeScript modules.
- `task test:mutation:go`: payment rules 6/6 killed; Food 52/55 killed, three
  survivors and no uncovered mutants. See [mutation scope and survivors](MUTATION-TESTING.md).

The engineering and protocol static reviews found and resolved payment-schema
runtime ownership checks, extensible provider identifiers, and the risk of
accidentally granting future real adapters simulated refund semantics. Their
reports are under `docs/reviews/development-payments-*.md`; these automated reviews
are not independent implementation evidence. Local Vite was restarted with the
development-payment flag and continues at `http://localhost:18181`.

## Public store browsing and table alignment

21 October 2026, Europe/Berlin (UTC+02:00). The homepage now lists this provider's
published stores with name search, scrollable results, and continuation links.
Customers can open a storefront and place a guest pickup order without a merchant
login or copied identifiers. This replaces the placeholder browse instructions;
ONIX is not a prerequisite for local browsing. The running development homepage
shows Döner Karlsruhe. Existing stores, menus, and credentials were preserved.

The UI uses the anonymous, modular OpenAPI `GET /api/v1/public/stores` contract.
Only visible published store names and identifiers cross its database projection;
ordinary tenant RLS remains intact. Search is literal and case-insensitive, and
bounded live pages bind their cursor to the trimmed query. This is a local
directory, not network-wide discovery. Public request limits and search scaling
remain release work. English, German, and Arabic messages cover the new controls.

The managed-stores table now aligns header and body cells explicitly at the
logical start edge, correcting browser-default centered headings in both LTR and
RTL layouts. Browser coverage checks English and Arabic alignment, empty and
filtered public results, hidden drafts/suspended stores, no-login navigation,
and search/navigation without JavaScript.

All three Docker browser journeys and `task smoke` passed. The backend part of
`task check` passed formatting, generated Go drift, vet, race tests, real
PostgreSQL integration, and the runtime build. The integration suite verifies
publication visibility, multi-merchant paging, changed-query cursor denial,
literal wildcard handling, tenant isolation, and the projection function's
ownership, search path, and execute privileges. Engineering and protocol static
reviews accepted the corrected bounded scope. The full check then caught a stale
TypeScript contract description after the cursor clarification; it was regenerated.
The final `task web:check` rerun passed generated-type drift, zero Svelte
errors/warnings, 25 unit tests, and the runtime build. These completed checks
satisfy the two static reviews' mechanical verification conditions.

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
