# Next interface improvements

Recorded 22 October 2026, Europe/Berlin (UTC+02:00). Remaining follow-ups are not
delivered features. Keep the restaurant journey
usable without turning this into a general dashboard redesign.

## Completed navigation and control fixes

- Store-level Menu and Orders tabs preserve the selected store, include a route
  back to the managed-store list, and remain visible on order details.
- Admin search/status/page-size fields share a baseline; Search and Clear filters
  occupy an aligned action row. Global button and inline-wrapper spacing are reset
  within that toolbar, without positional offsets.
- Clear filters and Refresh status are recognizable secondary controls.
  They remain navigation links styled as buttons, retaining no-JavaScript
  navigation and accessible names.
- Merchant workspace links use a readable shop name plus the persistent store
  handle, such as `/merchant/stores/doener-karlsruhe-store-1`. Resolution uses an
  exact tenant-scoped API lookup, not a scan of every store or a new database path
  in the UI. UUID API identity remains unchanged. Pre-launch UUID web links are
  not retained; enter stores through the merchant list. Custom domains remain
  future work. Customer storefront and receipt links now use the same readable
  store slug and a human-readable order reference; see the completed batch below.

## Completed order detail improvements

- Customer and merchant order details share a compact confirmed-event timeline
  with Lucide icons, readable labels, aligned timestamps, logical-direction
  layout, and no default list indentation. The API does not expose actor context,
  so the UI does not invent it. Selection, total, and separate operational states
  appear in a structured summary rather than a run of unrelated paragraphs.

## Automatic order updates

The founder rejected adaptive polling. Customer order status and merchant order
inbox/detail views now use SSE change hints, followed by ordinary authorized
reads. The server uses committed PostgreSQL notifications; the UI does not run
a timer to poll order state. A fresh stream requests a current read, including
after reconnect. Concurrent hints coalesce into one in-flight read and a trailing
refresh when needed. A source is closed while hidden and disposed on navigation.

Connection state and manual refresh remain visible. Hidden-tab/background alerts
need a separate notification strategy; this is not an always-on restaurant
terminal. See [live status design](LIVE-ORDER-UPDATES.md) for OpenAPI endpoints,
privacy, revocation, connection limits, and deployment follow-ups. No broker or
production cloud is selected. Normal authenticated JSON reads serve API and
future MCP clients without requiring SSE. Actual MCP tools remain future work.

Primary technical references consulted on 22 October 2026:

- [MDN Page Visibility API](https://developer.mozilla.org/en-US/docs/Web/API/Page_Visibility_API)
  describes visibility events and avoiding unnecessary background work.
- [MDN server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)
  describes one-way server-to-browser events; it does not establish an OCP
  deployment-cost advantage over polling.

## Readable customer order URLs

Implemented shape: `/stores/doener-karlsruhe-store-1/orders/O-000042`, with
`/stores/doener-karlsruhe-store-1` for the storefront. Exact API lookups resolve
the provider-local handle and the guest-owned reference to stable IDs. Names
can repeat or change without changing the handle; old name prefixes still work.
The user explicitly requested no legacy pre-launch UUID-route compatibility.
Custom storefront domains and subdomains remain future work.

Readable references are guessable navigation labels, never authorization. Retain
the current guest/registered-customer access checks and tenant scope. An unrelated
visitor who guesses the URL must not see private order details. Keep UUID-based
API identity where appropriate; a friendlier browser URL does not require
replacing primary keys or weakening the order contract. The merchant inbox now
uses an aligned, horizontally scrollable table with separate order, payment, and
pickup state columns, following the provider administration table style.

## Simple ordering and a real cart

Before this slice, use the [48-product demo](DEMO-RESTAURANT.md) and the
[restaurant task benchmark](RESTAURANT-UX-BENCHMARK.md). The public-menu comparison
exposes missing categories/descriptions and food-information concerns. Resolve
the model additions deliberately rather than masking them with UI-only fields.
The founder prefers categorized photo tiles with an optional compact list view.
Both presentations should share configuration, cart state, and pricing; use
progressive disclosure for product choices and a text-first fallback without photos.
The larger-menu browser walkthrough also exposes a confusing current state:
after quoting a configured pizza, the quote retains that pizza but the selection
form returns to the first menu item. Preserve the active item and choices in the
cart/configuration slice; a valid quote alone does not make that interaction clear.

The storefront currently submits one item configuration at a time to a quote;
it is not a multi-item cart. The Food selection contract already accepts up to
20 lines. Add a store-scoped cart with quantities, sizes, paid/free extras,
edit/remove actions, and a clear total, followed by an authoritative quote before
placement. Browsing and adding to a cart create no purchase commitment, payment,
or preparation work. Preserve that distinction when improving the screen.

Plan a cohesive, mobile-first UI/UX pass before a merchant pilot: product tiles,
simple category navigation, a visible cart, clear checkout/status, and a lean
merchant workspace. Do not postpone operational usability until final decoration.
Ordering controls now provide pause/resume and manual/automatic acceptance through
the Orders view and maintained APIs. Timed reopening and availability push updates
remain follow-ups. Next, exercise merchant setup through a thin MCP adapter and
Inspector before embedded chat; see [the plan](API-ERRORS-AND-AGENT-PLAN.md).
Continue category/presentation contracts and the shared tile/list cart afterward.
Keep native ONIX publication and
peer discovery as an explicit joined-network milestone, not an indefinite UI
follow-up or a claim already fulfilled by the local directory.

## Benchmark the restaurant work before extending the editor

Review primary merchant guides and public interface examples from Lieferando,
Wolt, relevant Delivery Hero brands where accessible, and Deliveroo or DoorDash. Describe
the actual task flow, not a checklist copied from marketing. Distinguish public
documentation from directly observed authenticated software; no partner accounts
or screenshots behind login have been inspected in this batch.

Compare store selection, menu/category organization, item editing, sizes and
extras, selection rules, availability/sold-out controls, preview/publishing, bulk
operations, error recovery, and order handling. Identify lean expected workflows
separately from paid extras/large-chain features. Check existing research before
duplicating the earlier restaurant-service review. Keep APIs and authorization
consistent with UI changes; do not expand the Food contract just for visual polish.

Initial primary-source leads, located 22 October 2026 (not a completed comparison):

- [Wolt menu management guide, German](https://merchant.wolt.com/de/deu/learning-center/manage-menu)
  covers items, options, categories, photos, prices, availability, and tabular edits.
- [Lieferando Partner Hub, Austrian edition](https://partnerinfo.lieferando.at/de/ihr-geschaeft/verwalten-sie-ihr-geschaeft-effizient-mit-dem-partner-hub)
  is a regional lead; verify German-market behavior before generalizing.
- [Deliveroo Menu Manager help collection](https://help.deliveroo.com/en/collections/19194841-menu-manager)
  provides task-level guides to examine.
- [Deliveroo Menu API overview](https://api-docs.deliveroo.com/docs/menu-api-overview)
  helps compare item/modifier/category semantics, not prove UI usability.

Before implementing, propose a bounded order of work and record checks in
`docs/STATUS.md`. Treat the user's "for later" request as a queued UX batch,
not as evidence that these follow-ups have already been implemented.
