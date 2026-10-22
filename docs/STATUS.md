# Development status

## Explicit commerce domain admission

22 October 2026. The first three-domain foundation batch separates installed
domain support from process configuration. Food is still the only installed
domain. Selecting `none` rejects catalog and purchase operations before storage,
while retaining shared store/account operations. Unknown, duplicate, and
uninstalled domain selections fail startup. Every OpenAPI path declares its
admission class; the enabled registry controls Food validation and projection.
Payment validates typed, server-derived financial terms, and HTTP telemetry
includes a bounded domain label without merchant or order identifiers.

Engineering and protocol static reviews identified and helped close gaps in
generic catalog admission and telemetry. Full `task check` and `task smoke`
passed. Gremlins killed all 5 capability and 14 payment mutants; Food had
67 killed and 4 surviving mutants, with none uncovered. The survivors remain
reported, not dismissed as equivalent. These checks do not establish
multi-domain isolation, UCP conformance, or independent scaling.

`docs/DOMAIN-BOUNDARIES.md` records the next implementation gates, particularly
exact-model filtering before installing another domain. `docs/FOOD-PRICING-V2.md`
records the reviewed size-dependent extra pricing and scheduled-offer design;
that model is not enabled yet. Native ONIX integration, complete Food V2 APIs,
and internal Shopping/Lodging journeys remain in the implementation plan.

## Generated artifacts stay out of Git

22 October 2026. The generated Go API, TypeScript API types, and Go/TypeScript
error catalogs are now ignored build artifacts, not tracked source. This
supersedes older entries describing committed generated code. OpenAPI sources,
error definitions, generator configuration, locked tool versions, and handwritten
handlers remain tracked. No custom implementation belongs in generated files.

`task generate` recreates the outputs; Go/web checks and Docker builds generate
their prerequisites. `task check` rejects these outputs if accidentally added to
the Git index. All four outputs were deleted locally and successfully regenerated.
Full `task check` passed, including Spectral, deterministic generation, Go race
tests, isolated PostgreSQL integration, 84 frontend tests, zero Svelte diagnostics,
and both runtime image builds with generated files excluded from their context.
`task smoke` passed after restarting the API; development data was not reset.
No browser regression suite was run for this build-policy change.

## Menu search focus styling

22 October 2026. Focus highlights the complete menu search field in brand blue,
replacing the brown rectangle around its inner input. Other focus indicators are
unchanged. All six browser journeys passed (38.2 seconds), including assertions
for the replacement focus styling; the focused mobile screenshot was inspected.
Full `task check` passed, including 84 frontend tests, zero Svelte diagnostics,
contract lint/drift checks, Go race/integration checks, and runtime builds.

## Compact menu controls and category filtering

22 October 2026. Customers switch between cards, list, and dropdown through one
floating view control at the bottom inline-start corner. A second control opens multi-select
category filters instead of jumping to headings. Selected categories combine;
search narrows that selection across all views. No category selection shows all
categories. A selected-category badge, live dish count, and clear-filters action
keep the applied state visible. Removable category chips share a row with the dish
count below search; the reset button aligns with the input. The view trigger is
square and icon-only, with labeled options and visible hover/selection feedback.
Controls dismiss on outside click or Escape.

Search stays at the top while scrolling and matches default/translated dish
names without accents or case: `Doner` finds `Döner`, and Arabic vowel marks
are ignored. Menu controls stay within the content width; narrow screens place
them above the basket/language row. Labels are available in DE/EN/AR, with
floating control positions and panel anchors mirrored for Arabic,
keyboard operation and isolated merchant-entered names. JavaScript-free item
links and single-item ordering remain; interactive filters require JavaScript.
No API contracts, payment behavior, or MCP capabilities changed.

Verification: full `task check` passed before the final visual refinements.
Final `task web:check` passed with 84 frontend tests, zero Svelte diagnostics,
generated-client drift checks, and a successful web build. All six browser
journeys passed (36.9 seconds), including alignment, icon-only view/hover states,
multi-category search, keyboard dismissal, narrow-screen bounds, Arabic mirroring,
and complete basket ordering. Mobile and desktop screenshots were inspected.
Development data was not reset; this is UI regression evidence, not user research.

## Spectral API design checks

22 October 2026. Root `.spectral.yaml` starts with the full Spectral OAS baseline
and adds path naming, camelCase path/query parameters, object-root JSON requests,
Problem JSON errors, explicit operation audiences, and body-free GET operations.
`task contracts:lint` runs pinned Spectral 6.17.0 and 14 rule tests in Docker;
`task check` includes it. The modular source currently has no lint warnings/errors.
Private publication metadata and exported code-generation aliases have four
documented baseline exceptions, not a blanket warning suppression.

Generated Go/TypeScript contracts, tenant middleware, and clients use camelCase
placeholder names together. Actual URL segments, JSON properties, stored records,
and replay bodies are unchanged. Declared search/filter query parameters remain
supported; the rule rejects query strings embedded in path keys. Successful SSE
responses and image redirects retain their appropriate media/body behavior.

Full `task check` and `task smoke` passed, including race-enabled Go tests,
database integration, generated-contract drift checks, 82 frontend tests,
zero Svelte diagnostics, and API/web builds. No development data was reset.
All six browser journeys passed (37.2 seconds), covering merchant publication,
guest ordering, provider moderation, localization, and direct image upload.
These checks do not establish third-party protocol conformance or production
payment-provider behavior. MCP implementation and admin redesign remain separate work.

## Item note spacing

22 October 2026. Expanded basket item notes now have a 12px gap below the
quantity/note/edit/remove controls and a small bottom margin. Collapsed notes
add no row height. A browser geometry assertion checks the gap; all six browser
journeys passed (37.4 seconds). Full `task check` also passed, including 82 frontend
tests and zero Svelte diagnostics. No API or MCP behavior changed.

## Multi-item ordering with automatic basket checks

22 October 2026, Europe/Berlin. Customers can now customize dishes in native
modals and build a real multi-item basket: quantities, edit/remove, dish notes,
and a separate whole-order note. A compact floating basket opens a modal with
fixed header/checkout and a keyboard-scrollable item region. Rows share one
quantity/note/edit/remove action line; descriptions wrap without smaller type.
The redundant bordered checkout panel is removed. Menu search and view controls
align, and sold-out dishes have a strong grayscale image and explicit badge.

Basket changes automatically validate totals and availability; no manual price
check or review action is required in the JavaScript flow. Short input debouncing,
serialized requests, stale-response rejection, and quote-expiry refresh protect
checkout without periodic polling. Explicit placement still uses scoped guest
authority, an idempotency key, and backend availability checks. Published menus
and ordering pause/resume changes now send public SSE hints. Live refresh retains
customer choices and open dialogs; unavailable selections block ordering.
The stream is separate from private order notifications and contains no order data.

Optional DE/EN/AR menu-name translations and merchant-declared default options
are editable/publishable data, not client guesses. Quote labels use the selected
language with fallback, keeping identifiers and amounts unchanged. Dish and order
notes survive placement into customer and merchant order details. Tab-local
session storage retains selections/notes across reloads, not prices or credentials.
Without JavaScript, the existing single-dish native form remains available.

The 48-item demo was enriched to revision 4 with translations and a declared
garlic-sauce default, preserving existing edits and photographs. Custom stores
were not reset. [Open the demo](http://localhost:18181/stores/marktplatz-kueche-demo-store-2).
The application remains pre-launch: production SSE proxy/admission policies,
internet abuse controls, and automatic-check quote-volume limits need release
assessment. Existing quote and guest capacities are not an abuse-protection claim.

Verification: full `task check` passed, including contract/generated-code drift,
Go vet/race tests, backend/web builds, 82 frontend tests and zero Svelte diagnostics.
Real PostgreSQL
integration tests passed, including committed/rolled-back availability hints,
private-channel separation, foreign-store isolation, and withdrawal. Six browser
journeys passed (42.6 seconds), exercising multi-item placement with both note
types, reload/edit/remove, no-JavaScript fallback, publication-driven availability,
pause/resume without reload, RTL, modal focus/scroll restoration, and fixed checkout
geometry on a narrow viewport. The mobile basket screenshot was inspected.
`task smoke` passed. Repeat demo enrichment was a no-op; a local read confirmed
48 items with photographs and translations at revision 4 and the sauce default.
No real payment or cloud service was exercised in this batch.

Merchant/customer MCP expansion remains next; provider-admin redesign has not
started. Notify the founder before that redesign for their screenshots. The
requested "Spectral for UI" tool needs clarification before selecting tooling.

## Categorized visual customer menu

22 October 2026, Europe/Berlin. The customer storefront now opens with image
cards grouped into merchant-defined categories. It also offers a compact list,
a grouped dropdown, menu search, and category jump links. Choosing a dish
reveals its sizes, extras, and quantity; keyboard focus follows that choice.
Price checking scrolls to the retained quote, and placement remains explicit.
Sold-out dishes remain visible. This increment preserves the single-dish quote
flow; it does not introduce a multi-dish cart.

Categories are real optional Food menu data, not fixture-only labels. Merchants
can add, rename, reorder, remove, and assign categories in their draft. Removing
a category leaves its dishes under Other dishes. Save and publish use the existing
authorized application operations. The modular schema, generated Go/TypeScript,
semantic validation, and public projection agree. Duplicate/blank categories and
dangling references are rejected; legacy uncategorized menus still validate.
This is a coordinated draft-contract change, not backward compatibility with old
closed menu validators. Category metadata does not affect prices or order terms.

The existing illustrated demo was enriched to revision 3 with seven categories;
all 48 item photos and other content were retained. A repeated enrichment was a
no-op. Custom stores were not reset. The running API was rebuilt and the native
Vite frontend continues to hot-reload. Open the [demo storefront](http://localhost:18181/stores/marktplatz-kueche-demo-store-2).

Verification includes category validation/projection and unchanged-price tests,
preserving seed tests, browser component tests, and the six updated browser
journeys. They exercise category editing/publication, cards/list/dropdown/search,
progressive customization, configured pizza pricing, sold-out state, narrow RTL
layout, no-JavaScript dish selection, and existing order/payment workflows.
The UI references and design choices are recorded in [UI-UX.md](UI-UX.md).
Final `task check` passed: 76 frontend tests, zero Svelte errors/warnings,
contract-generation drift checks, Go race/vet tests, real PostgreSQL integration,
and runtime builds. `task test:browser` passed all six journeys (31.7 seconds),
including decoded photo checks in both the card and customization views.
`task smoke` passed; a repeated demo enrichment added no revision. Local HTTP
reads confirmed revision 3, seven categories, 48 items, 48 photo references, and
successful JPEG delivery. A separate Docker-to-native-Vite visual probe needed
hostname translation and could not decode its translated image response; it is
not counted as a passed visual check. The normal local image URL and the
disposable browser journey both delivered the photo successfully. Browser
screenshots of the photo card and narrow RTL menu were inspected separately.
Merchant/customer MCP expansion follows this slice. Admin redesign has not
started; notify the founder first for reference screenshots.

## Authorized menu photos and complete demo assets

22 October 2026, Europe/Berlin. All 48 fictional restaurant products have
matching generated photos, optimized to at most 768 pixels per side. The
committed JPEG set totals 6,263,301 bytes. Exact prompts and manifest hashes
are retained; original PNGs remain ignored. Image generation ran in a separate
agent while the API, database, upload UI, and tests were implemented.

Media now has OpenAPI-defined allocation, metadata, short-lived upload grants,
completion, private preview, and publication-checked public image resolution.
Uploads go directly from the browser to private S3-compatible storage. The
server checks stored bytes and dimensions and produces a metadata-free JPEG
derivative. Menu revisions retain store-owned ready media IDs, never signed
URLs. Tenant isolation, scoped permissions, quotas, and durable replay apply.
Merchant photo controls and customer dish display are available in English,
German, and Arabic. This is not the planned menu tiles/cart UX redesign.

`task dev:seed-media` attached and published 48 photos in demo revision 2,
preserving the other menu data. A repeat reported no missing photos and made
no additional revision. Existing custom stores were not reset or illustrated
with unrelated demo photos. The updated local API is running; native Vite
continues to hot-reload the frontend.

Executed checks: full `task check` (73 frontend tests, no Svelte diagnostics,
race/vet/PostgreSQL/contract drift and runtime builds); repeated `task media:check`
including actual stored-object processing; focused fixture completeness,
provenance/hash/dimensions and image-validation race tests; `task smoke`.
`task test:browser` passed all six journeys. The photo test covered direct PUT, checked preview, draft save, unpublished
image denial, publication, and customer display. Initial browser attempts
exposed missing Web Crypto in Chromium headless-shell on Docker HTTP origins;
the harness now uses full headless Chromium with an explicit test-origin-only
secure-context flag. This is not a relaxation of production HTTPS requirements.
The photo test also withdraws its own test publication during cleanup, preserving
the empty-directory precondition of the separate publication journey.
No R2/cloud storage test ran. See [MEDIA.md](MEDIA.md) for limits, including
cleanup/retention still to implement and bounded signed-URL expiry.

Stages 1–2 of [the media/MCP plan](MEDIA-AND-MCP-PLAN.md) are complete. The
seven existing merchant MCP tools are unchanged; additional setup/publication
intentions, customer MCP, and model-driven chat are still pending. Admin
redesign has not started; notify the founder before it so their screenshots
can inform the APIs/MCPs management areas.

## Presigned media foundation and MCP feature assessment

22 October 2026, Europe/Berlin (UTC+02:00). Optional private local object storage
now runs through the `media` Compose profile using the supplied pinned SILO
digest, its own named volume, and loopback ports 18900/18901. Three synthetic
demo food photos (Döner, Margherita, falafel) are retained with exact prompts,
checksums, and item associations under `assets/demo/food/`; optimized JPEGs total
840,012 bytes. They are prepared assets, not yet visible on a storefront.

The Go `internal/media` signer uses pinned AWS SDK v2 modules. Upload bytes go
directly from the client to storage using short-lived presigned URLs bound to
type, size, key, and SHA-256. No provider upload proxy, storage credentials in
clients, or custom signature algorithm was added. The storage primitive is not
yet an authorized media API. Menu attachment, completion verification, image
validation/derivatives, and UI/MCP presentation are the next media slice.

`task media:check` passed twice-seeded race-enabled SILO tests: direct PUT/GET,
byte comparison, wrong-checksum/type/key rejection, expired URL denial, private
object/list access, and accepted/rejected CORS origins. Focused unit/race tests,
vet, and `task doctor` passed. Full `task check` passed, including real PostgreSQL
integration, race tests, generation drift, zero Svelte diagnostics, 70 web tests,
and API/web builds. `task smoke` passed on the rebuilt runtime. No real browser upload or cloud storage
test ran, and no development menu was changed or reset.

[Media notes](MEDIA.md) document the upload boundary, inspected OpenOM pattern,
and R2's current free-tier/cost direction without selecting the production cloud.
[MCP findings](MCP-MEDIA-AND-INTERACTIONS.md) distinguish ordinary image results
from Apps, application basket handles from transport state, cacheable resource
reads from arbitrary tool results, and Tasks from restaurant order progress.
Those richer MCP interactions are not implemented by the seven current tools.

Admin redesign has not started. Notify the founder before it, especially the
APIs/MCPs areas, so their offered screenshots can inform the design.

## Focused merchant price edits and draft review

22 October 2026, Europe/Berlin (UTC+02:00). The local merchant MCP now has
seven tools. `update_item_prices` changes up to 20 distinct size prices or extra
surcharges on one dish in one private revision. `review_menu_changes` compares
two retained revisions in pages of 20 field changes, including dish IDs/names
and before/after values. REST/UI edits are included in the comparison. These
use the existing Offering operation and grant policy, not a second menu store
or arbitrary API dispatcher. Publication remains an explicit workspace action;
comparison does not record human approval or change the live menu.

Full `task check` passed, including PostgreSQL integration, Go race/vet,
contract-generation drift, Svelte diagnostics, 70 web tests, and the API build.
Focused race tests, vet, and the real HTTP/SQL MCP test were repeated after
the final review-context refinement. They cover price preservation, free extras,
atomic rejection of missing/duplicate targets, fractional/negative price denial,
stale revisions, retries, tenant scope, and stable 48-item comparison pagination
after a newer draft is saved. `task smoke` passed on the rebuilt API. Inspector
2.12.0 strict discovery passed for seven tools without warnings and executed a
read-only comparison. Development menus were not changed or reset; mutations
ran in disposable test databases. No model, browser suite, or load test ran.

Menu creation/import, other corrections, approval/publication tools, customer and
account tools, production OAuth, embedded chat, and admin implementation remain.
Before starting the admin redesign, especially APIs/MCPs, notify the founder
so their reference screenshots can guide it. No admin screen changed here.

## Progressive merchant menu information

22 October 2026, Europe/Berlin (UTC+02:00). The merchant MCP now has five tools:
the existing store lookup and draft availability change, searchable menu
summaries, dish detail, and selected extras detail. Sizes/prices arrive together;
one extras call can batch up to three relevant groups. Reads can go directly to
a known item and do not force a wizard. This implements progressive disclosure
of information, not progressive tool-definition loading. The
[tool design](MCP-TOOL-DESIGN.md) records remaining merchant/customer intentions
and the trade-off between context size and round trips.

The prepared known-store scenario uses three calls and returns 3,429 serialized
result bytes including text/structured copies, versus a 21,393-byte source menu;
the discovery result is another 4,314 bytes. These are diagnostics, not a
tokenizer measurement, comparable full HTTP exchanges, or a model quality/speed
claim. Actual model evaluation remains future work.

Full `task check` passed, including Go vet/race, real PostgreSQL integration,
70 web tests, Svelte checks, generated-contract drift, and builds. Additional
focused race tests, vet, and an actual HTTP/SQL MCP test passed after tightening
the extras input's nullable-array schema. Tenant denial, revision consistency,
free/paid choices, bounded reads, and invalid selectors/batches are covered.
`task smoke` passed after rebuilding the final local API. Inspector 2.12.0 passed
five-tool strict schema inspection without warnings and actual item/extras calls.
No development data
was reset and no model was called. Browser/load suites were not rerun.

Admin remains a later slice. The [UI conventions](UI-UX.md) now record the
dedicated full-page admin workspace and compact, accessible search/filter toolbar
ideas, including active-filter indicators. No screen changed in this batch.

## Provider administration additions recorded

22 October 2026, Europe/Berlin (UTC+02:00). The requested future tab order is
**Merchants, Stores, Orders, APIs, MCPs**. The
[admin plan](OBSERVABILITY-AND-MCP-OPERATIONS.md#provider-administration-navigation)
records searchable merchants, API documentation/schema links, runtime health,
and per-endpoint operational measurements. MCPs remains management as well as
monitoring. Documentation only: no UI/API/runtime changes or application tests
in this checkpoint. Implement these with the planned admin work, not as empty tabs.

## Local merchant Streamable HTTP MCP foundation

22 October 2026, Europe/Berlin (UTC+02:00). The local-only `/mcp/merchant`
endpoint now exposes an initial three-tool test slice: find owned stores,
inspect bounded menu summaries, and change item availability in a private draft.
The mutation shares the REST Offering operation, preserving variant/extra data,
revision checks, and retry behavior. Separate MCP and REST bearer resources
share merchant ownership, scopes, expiry, revocation, and application policy.
This is development authentication, **not OAuth** or a public deployment boundary.
See [operation and limitations](MCP.md) and the [tool contract](../contracts/mcp/v1/merchant/README.md).

The three tools are not the final merchant toolset. Before expanding it, review
merchant/customer intentions, progressive disclosure of schemas and results,
and full-workflow token cost. Item detail, menu creation/import, focused
corrections, publication review, account tools, and the customer MCP remain
work to design and implement. The **MCPs** admin area and embedded chat also
remain pending. No model or paid API call was made.

Tool-level OTel spans and bounded call/duration/in-flight metrics complement
HTTP instrumentation, including tool rejections returned inside HTTP 200.
Payloads, credentials, and commerce identities are not emitted as tool labels
or failure logs. Export remains disabled by default; no collector was added.

Verification covers scope-filtered discovery, cross-merchant denial, resource
separation, expiry/revocation, immutable-base retries, conflicts, unchanged
variant/extra fields, and 48-item pagination. MCP Inspector 2.12.0 exercised
modern and legacy clients against the local HTTP endpoint; strict tool-schema
checks and a bounded menu read passed. These are local tests, not independent
operator interoperability or an AI task-quality evaluation. Development data
was not reset. `task check` passed, including contract drift checks, Go vet/race,
real PostgreSQL integration, 70 web tests, Svelte checks (zero errors/warnings),
and API/web builds. `task smoke` passed after rebuilding the local API with the
final code. Browser and load suites were not rerun.

## OpenTelemetry runtime foundation and MCPs naming

22 October 2026, Europe/Berlin (UTC+02:00). The requested future admin tab is
named **MCPs**, covering management as well as monitoring. It is not yet a UI tab:
neither MCP endpoint has been implemented. The Go provider now has OTel 1.47.0
HTTP server spans, request/duration/in-flight metrics, and safe correlated failure
logs. Optional OTLP/HTTP export uses bounded, non-blocking trace buffering and
short export timeouts. Export is disabled by default; no monitoring backend is
required or installed. See [operation and scope](OBSERVABILITY.md).

Route templates and registered codes keep labels bounded; raw paths, request and
response bodies, credentials, baggage, tracestate, and commerce identities are
not collected. This batch does not change authentication or implement OAuth,
tool-level spans, database/payment child spans, retained statistics, or an admin
monitoring API. Shared authorization and intent-based MCP tools remain next.

Passed: focused race-enabled telemetry/HTTP/API tests, `task check` (including
contract drift, Go vet/race, SQL integration, 70 web tests, Svelte checks, and
runtime builds), a further `task test:integration` with real merchant/customer
SSE streams passing through telemetry, Compose validation, and `task smoke`.
Tests exercised a local OTLP/HTTP receiver and a failing receiver, not Grafana
or a production backend. No browser suite or load test was rerun; no development
data was reset. The rebuilt local API is running with export disabled.

## MCP operational visibility and OpenTelemetry

22 October 2026, Europe/Berlin (UTC+02:00). The
[observability plan](OBSERVABILITY-AND-MCP-OPERATIONS.md) adds an MCPs tab
to the upcoming merchant/customer MCP work. It specifies call/error/latency
visibility, safe recent failures, bounded storage, and OTel traces/metrics without
a mandatory Grafana deployment. Shared API/MCP authorization precedes the tools;
existing bearer grants are not OAuth. This is a documentation-only checkpoint:
no MCP endpoint, monitoring UI, OTel dependency, or runtime instrumentation was
added, and no application tests were rerun.

## Intent-based MCP surfaces and authorization

22 October 2026, Europe/Berlin (UTC+02:00). The
[surface clarification](MCP-SURFACES-AND-AUTHORIZATION.md) records four logical
tool groups: merchant commerce, merchant account, customer commerce, and customer
account. Recommend two audience-specific HTTP endpoints in one runtime, not a
REST export or four required deployments. Tools need task-oriented inputs and
compact outputs, with per-call authorization beneath filtered discovery.

The x401 draft 0.2.0 assessment uses a pinned source commit: credential proofs can
later inform authorization but do not supply our account permissions, delegation,
or payment authority. Documentation-only; no authentication system or tools were
implemented, and no x401 or application tests were run. Earlier agent-plan wording
has been corrected to prevent an endpoint-wrapper implementation.

## Streamable HTTP and chat research

22 October 2026, Europe/Berlin (UTC+02:00). The
[product investigation](MCP-AND-CHAT-RESEARCH.md) recommends Streamable HTTP MCP
in the existing Go provider, then Vercel AI SDK/SvelteKit onboarding through that
HTTP tool boundary. It supersedes the stdio-first plan. Feature coverage takes
precedence over stack preservation; official documentation shows no current
reason to replace SvelteKit. The compatible package release set, authorization
implementation, and end-to-end feature support still require execution checks.

The note records protocol-era differences, scoped authentication without token
passthrough, explicit review before publishing, model-free Inspector tests,
separate API billing, and Infisical EU as the preferred later secrets manager.
This is documentation/source research only: no dependencies, secrets, application
behavior, cloud services, or candidate-review ledgers changed. No application
test suite was rerun for this documentation-only batch.

## Ordering controls and the early agent-facing plan

22 October 2026, Europe/Berlin (UTC+02:00). The store Orders tab now exposes
pause/resume and manual/automatic acceptance through three maintained OpenAPI
operations. Existing stores default to open/manual, preserving existing behavior.
Pause keeps browsing and existing purchases available, but blocks new quotes and
first placements, including old quotes. Successful placement retries still return
their original result. Settings use their own revision, scoped authorization,
idempotency, and append-only actor/request audit, serialized on the existing store
lock. Automatic acceptance records the merchant policy's authorizing actor and
does not collect payment or start preparation. Later policy changes leave old
orders alone. Public settings exclude the private actor and audit.

The [API and agent plan](API-ERRORS-AND-AGENT-PLAN.md) records the error-boundary
audit and moves a local merchant-setup MCP/Inspector slice ahead of embedded chat,
without waiting for ONIX or final UI polish. Stable module-owned error codes,
Problem Details, schema checks, scope/tenant checks, safe Fluent rendering, and
generated artifacts already exist. The audit corrected generic problem titles
to HTTP status phrases and added envelope regression checks. Safe field-level
validation pointers, operation-specific code inventories, and internal versus
transient failure classification remain improvements, not delivered features.

Verification passed: `task check` (real PostgreSQL integration, Go race checks,
generated drift checks, 70 web tests, and zero Svelte diagnostics), all five
built-app browser tests, all five Vite browser tests, and `task smoke`.
The large-menu browser journey now pauses ordering through the merchant UI,
checks disabled customer quoting, resumes with automatic acceptance through the
API, and places a configured pizza order. The real-DB test checks policy defaults,
scope denial, malformed mode, stale revision, pause enforcement, resumed old-quote
placement, actor attribution, unchanged payment/preparation, historical replay,
non-retroactive settings, publication withdrawal, and audit counts. The new
controls were inspected at 390-pixel width.

Two test issues were resolved: the historical upgrade fixture called today's
quote handler against the deliberately old schema, and a new browser selector
incorrectly required exact text on a select's enclosing label. The historical
fixture now constructs priced historical terms before testing the real migration
and retained replay; no production compatibility fallback was added.

Local API/database were rebuilt/migrated without resetting data; native Vite
returned HTTP 200 and retained the substantial demo listing. No new mutation
score, real payment, MCP execution, or joined-network result is claimed.
Timed reopening and pushed availability updates remain follow-ups. Category/
presentation contracts, progressive product configuration, the multi-item cart,
and the known post-quote selection reset remain outstanding UI work.

## Substantial demo menu and customer UX research

22 October 2026, Europe/Berlin (UTC+02:00). An explicit `task dev:seed` adds a
fictional 48-product restaurant under its own merchant. Repeating it preserves
edits, withdrawal, purchases, credentials, and existing stores. It is not a data
migration or automatic startup reset. The local demo is currently `store-2`;
`store-1` remains Döner Karlsruhe. Use `task dev:stores` to inspect local ownership,
then `task dev:credential STORE=store-2` (or `MERCHANT=<UUID>`) to issue a 24-hour
credential for that merchant. Credentials are not global; unknown selectors fail
instead of creating a merchant. Omitting selectors still creates a new merchant,
with an explicit warning. No production identity/login mechanism is implied.

The fixture covers sizes, required sauce/side choices, free/paid extras, and a
sold-out item. Real PostgreSQL integration checks tenant isolation, repeat-seed
preservation, a two-line €33 quote, and rejected unavailable/incomplete selections.
A separate three-product public-menu mapping validates limited real data shapes;
it is not a lossless real-menu import. Categories remain fixture metadata because
the current closed menu contract cannot yet represent them. Descriptions, photos,
allergens, and deposit breakdowns are also explicit gaps before pilot acceptance.

[The UX benchmark](RESTAURANT-UX-BENCHMARK.md) records seven merchant/menu sources
and four additional customer-UX sources, including 2025 form guidance and a 2026
food-delivery benchmark. Proposed browsing uses categorized photo tiles and a
compact list, with one shared configuration/cart flow. Select a dish, show required
choices, then optional extras and the configured total. Simple fixed items may
later offer quick-add. This is evidence-informed design, not our own usability
study. The operating controls and cart remain the next two implementation slices.

`task check` passed, including real PostgreSQL integration, Go race tests,
generated-contract drift checks, 70 web unit tests, and zero Svelte diagnostics.
Local seeding, repeated seeding, store listing, and selected credential issuance
were executed successfully. Native Vite returned HTTP 200 and listed the demo.
All five built-app browser tests passed after correcting two new test assertions:
the accessible name is `Menu item`, and a native option's disabled DOM property
needs an explicit assertion. The large-menu journey also passed in Vite after
explicitly waiting for hydration before client-bound selection. A separate Vite
run exposed an existing language-switch test reading the page during navigation;
it now waits for the target language and hydration before measuring layout.
All five Vite browser tests passed on the final rerun. `task smoke` passed without
resetting data. The built-app suite passed before these final synchronization-only
test adjustments; no application UI code changed in this batch.
Desktop merchant and narrow customer screenshots were inspected. The latter
exposes a current post-quote form reset, recorded for the cart/configuration slice;
the validated quote retains the selected pizza. The current UI is not the proposed
tile/list design. No new mutation or production usability tests are claimed.
See [demo instructions](DEMO-RESTAURANT.md) for login and walkthrough examples.

## Order detail timelines and SSE updates

22 October 2026, Europe/Berlin (UTC+02:00). Customer pickup receipts and merchant
order details now share a
structured order summary and a confirmed-event timeline with Lucide icons,
timestamps, and logical-direction styling. Receipt introduction text no longer
claims that a progressing order has never reached payment or preparation.

The founder rejected adaptive polling. Optional, OpenAPI-defined SSE endpoints
now complement ordinary authorized order reads. PostgreSQL event-insert
notifications signal committed changes through one listener per API process;
no separate broker or periodic order-state poll is added. The same-origin web
proxy retains bearer credentials server-side. The merchant inbox and merchant/
customer detail screens subscribe while visible, coalesce refreshes, and retain
manual refresh. Reconnection requests current state, not event-stream replay.
See [live status design](LIVE-ORDER-UPDATES.md) for API semantics, privacy,
resource boundaries, and hosting follow-ups. No MCP server is implemented.

Verification: `task check` passed, including generated-contract drift, Go checks,
real PostgreSQL integration, 70 web unit tests, and zero Svelte diagnostics.
After a final bounded authentication-timeout adjustment, `task fmt`, `task vet`,
`task test:race`, and `task test:integration` passed again. All four built-app
browser tests and all four Vite browser tests passed. The pickup journey verifies
cross-session updates without manual refresh, including a new inbox arrival and
all seven recorded history entries. Desktop and narrow Arabic screenshots were
inspected. `task smoke` passed and updated the local API to migration 11 without
resetting development data; native Vite remains on port 18181.

Initial checks exposed a ServeMux conflict with readable-order lookup, an overly
strict EventSource test type, and a test that left Arabic selected before expecting
an English login label. These were corrected and the affected checks rerun;
the guest stream uses the distinct `/food/order-updates/{orderId}` path.
This batch does not change real payment adapters, the cart, or native ONIX
integration. Mutation tests and production load/failover tests are not claimed.
Next: pause/resume ordering and manual/automatic acceptance, followed by the
planned cart and cohesive ordering UI. The broader UI/UX pass remains necessary.

## Readable customer links and a usable merchant order table

22 October 2026, Europe/Berlin (UTC+02:00). Customer storefronts now use
`/stores/doener-karlsruhe-store-1`; receipts and development checkout use
`/stores/doener-karlsruhe-store-1/orders/O-000042` and its `/checkout` child.
Homepage, merchant storefront links, placement redirects, refresh, checkout,
and receipt navigation use these routes. UUIDs remain the primary API identities.
Old pre-launch UUID web routes are removed by founder request, not redirected.

A new modular public handle-resolution operation reads one indexed, visible
store projection. Migration 10 retains the narrow definer-function privilege
boundary: no draft, withdrawn, or suspended store is exposed. A guest-only
reference lookup retains the existing store/merchant/guest authorization and
current order/history read. Guest cookies cover `/stores`, with a small HttpOnly
routing hint so receipts do not depend on public listing or a store-name prefix.
The hint and readable reference confer no authority. Existing development data
is preserved; older guest links/cookie paths are not migrated.

The merchant Orders tab now has a card and aligned table matching administration:
reference, placement time, total, order status, payment, and pickup progress.
References open details; a visible secondary refresh control returns to the
latest inbox. Logical-direction alignment, readable status text, and a keyboard-
focusable horizontal scroll area support narrow screens and Arabic.

Verification: `task check` passed, including generated-contract drift, Go checks,
real PostgreSQL isolation and alias/withdrawal/revocation tests, 63 web unit tests,
and zero Svelte errors or warnings. `task smoke` passed. The running local
API uses migration 10 without resetting its data, and native Vite serves 18181;
homepage and readable storefront both returned HTTP 200. The initial four
built-app browser tests passed, including the existing payment/cancellation/pickup
journey and table geometry. Desktop and narrow Arabic screenshots were inspected.
All four Vite browser tests also passed on a bounded rerun. The first Vite run
passed three tests but timed out opening a merchant workspace; the cause was not
established, and no timeout increase or application workaround was introduced.
No mutation-testing, real-payment, or native-network result is claimed for this batch.

Next: automatic updates for customer order pages and merchant inbox/detail views,
then resume pause/resume ordering and manual/automatic acceptance. The UI still
lacks a multi-item cart: the API supports up to 20 selection lines, but the current
web form submits one configured item. A cohesive tile/category/cart/checkout UX
pass is required before a merchant pilot, not just cosmetic work at the end.
See [the UI backlog](UI-UX-NEXT.md) for these boundaries and refresh behavior.

The implementation remains a single-provider pickup slice with simulated payment,
not the completed network. `services/onix/` currently contains integration notes,
not a running native publisher/crawler. The joined-network milestone remains
two providers, native publication/crawl into discovery, freshness/withdrawal and
authorized routing, and separately scoped shared payment integration. Real payment
adapters, own delivery, registered customer journeys, and production authentication
also remain work. Prior research/POCs do not count as delivered product features.

Valueflows continues to inform the separation between offerings, agreed purchase
terms/accepted obligations, and reports of actual fulfilment or payment. The
implementation is not a Valueflows ontology or conformance implementation. A future
cart remains a revisable selection, not an accepted obligation or economic event.

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
