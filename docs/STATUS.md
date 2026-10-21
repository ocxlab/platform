# Development status

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
