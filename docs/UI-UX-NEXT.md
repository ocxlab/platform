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
  not retained; enter stores through the merchant list. This does not yet change
  customer storefront or receipt URLs or implement custom domains.

## Remaining navigation improvements

- Rework order history as a compact event timeline with Lucide icons, clear event
  labels, actor/time context where available, aligned content, and no default list
  indentation. Do not use color or icons as the sole description of an event.

## Automatic order updates

Manual-only refresh is a gap, not the intended finished customer experience.
Prioritize customer order status and merchant order inbox/detail views.

Evaluate adaptive polling first against the existing authorized read APIs: a
candidate interval is approximately five seconds for a visible active customer
order, with an immediate refresh on return to the page. This is a design proposal,
not an industry-mandated interval or a performance result. Avoid overlapping
requests, back off on errors, and avoid replacing unsaved forms or stealing focus.
Stop or slow updates only when both order and relevant payment/refund activity no
longer need watching. A cancelled order can still have financial follow-up.

Use page visibility to avoid waste, but treat the merchant live inbox separately:
background order alerts need an explicit notification strategy, and a hidden or
suspended browser tab is not reliable always-on infrastructure. Show connection
state/last update and retain manual refresh as a fallback. Assess conditional or
revision-based reads without assuming a 304 response eliminates server/database
work. Test refresh after acceptance, payment, preparation, readiness, and pickup;
also stale credentials, offline/reconnect, and interrupted updates.

SSE is a candidate for server-to-browser change notifications as concurrency and
latency needs justify persistent connections. Notify only authorized audiences;
refetch authoritative records after a hint/reconnect and respect replay/cursor
boundaries. WebSockets remain an option where genuine bidirectional streaming is
needed; ordinary purchase commands can stay on REST. Choose by deployment and
measured workload, not by which transport sounds newest. No broker, mandatory
WebSocket runtime, or production cloud is selected here.

Primary technical references consulted on 22 October 2026:

- [MDN Page Visibility API](https://developer.mozilla.org/en-US/docs/Web/API/Page_Visibility_API)
  describes visibility events and avoiding unnecessary background work.
- [MDN server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)
  describes one-way server-to-browser events; it does not establish an OCP
  deployment-cost advantage over polling.

## Readable customer order URLs

Use a readable store slug and human-readable order reference on customer-facing
receipt/status routes, rather than exposing a chain of merchant/store/order UUIDs.
For example, `/stores/doener-karlsruhe/orders/O-000042` illustrates the intended
shape; it is not yet the chosen or implemented route. Resolve these aliases to
stable internal IDs. Store-name collisions, later renames, redirects from existing
links, and custom storefront domains need deliberate handling; a display name
alone is not a unique identifier.

Readable references are guessable navigation labels, never authorization. Retain
the current guest/registered-customer access checks and tenant scope. An unrelated
visitor who guesses the URL must not see private order details. Keep UUID-based
API identity where appropriate; a friendlier browser URL does not require
replacing primary keys or weakening the order contract. Add direct-link, refresh,
rename/old-link, and denied-access regressions when this is implemented.

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
