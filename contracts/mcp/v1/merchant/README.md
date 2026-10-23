# Merchant tool contract

The merchant MCP helps owners find a managed store, inspect its menu, change
availability, prices, and meal offers in a private draft, and review retained
changes. It is an intention-oriented interface, not an export of every REST
operation. Publication remains a separate
separate confirmed publication action, so a draft edit cannot change the live menu.

The endpoint is `/mcp/merchant`. Its local implementation uses the released Go
SDK's Streamable HTTP transport. This developer slice is not a public OAuth
resource server. See [setup and boundaries](../../../../docs/MCP.md).

## Tools and permissions

| Tool | Required merchant scopes | Input | Compact result |
| --- | --- | --- | --- |
| `find_my_stores` | `store:read` | Optional `after` continuation | Up to 20 names, handles, draft revisions, and item counts; optional `next_after` |
| `inspect_menu` | `offering:read` | `store` handle; optional literal `query`, `revision`, and `offset` | Up to 20 matching item summaries, currency, revision, whole-menu `total_items`, `matching_items`, and optional `next_offset` |
| `inspect_menu_item` | `offering:read` | `store`, exact `item`; optional `revision` | One dish's availability, all variant IDs/names/prices, and extras-group IDs/names/limits/defaults/counts, not option lists |
| `inspect_item_extras` | `offering:read` | `store`, exact `item`, positive `revision`, one to three distinct `groups` IDs | The selected groups with choice limits/defaults and all option IDs/names/base and size-specific surcharges, preserving requested group order |
| `set_item_availability` | `offering:read`, `offering:write` | `store`, exact `item` ID, boolean `available`, positive `expected_revision`, `request_key` | Saved draft revision, item availability, replay flag, and next action; no full menu echo |
| `update_item_prices` | `offering:read`, `offering:write` | `store`, exact `item`, positive `expected_revision`, `request_key`, and 1–20 distinct `changes` | Base and saved draft revisions, replay flag, and next action; no menu echo |
| `review_menu_changes` | `offering:read` | `store`, positive `from_revision` and `to_revision` at or after it; optional `offset` | Up to 20 before/after field changes, total count, explicit revision pair, and optional `next_offset` |
| `inspect_meal_offers` | `offering:read` | `store`; optional exact main `item` filter or exact `offer` detail selector; `revision` and `offset` | Up to 20 offer summaries, or one offer's terms and up to 20 named add-on choices with draft availability; optional `next_offset` |
| `save_meal_offer` | `offering:read`, `offering:write` | `store`, positive `expected_revision`, `request_key`, complete `offer`; explicit `create: true` for a new ID | Base and saved draft revisions, offer ID, replay flag, and review instruction; no menu echo |

Scopes are checked in discovery and invocation. Every HTTP request authenticates
the grant again, including expiry and revocation. Store handles select records;
they grant no authority. Merchant ownership and PostgreSQL tenant policies remain
in force. Provider-admin and guest credentials are not merchant credentials.

The schema-authoring source is the typed inputs in
[`agenttools`](../../../../services/platform/internal/food/adapters/mcp/merchant.go)
and its [menu detail inputs](../../../../services/platform/internal/food/adapters/mcp/menu_details.go).
Focused edits and comparison inputs are in
[`menu_edits.go`](../../../../services/platform/internal/food/adapters/mcp/menu_edits.go).
Meal-offer inputs and projections are in
[`meal_offers.go`](../../../../services/platform/internal/food/adapters/mcp/meal_offers.go).
The pinned SDK derives and validates JSON Schemas, exposed by `tools/list`.
The extras input uses a narrow schema refinement to advertise a non-null array;
field definitions and descriptions still derive from its typed input.
REST remains defined by the modular OpenAPI sources; MCP schemas do not replace
those contracts. Schema portability is checked by `task mcp:inspect`.

## Editing and retries

`set_item_availability` changes only the selected item's `available` field. It
retains variants, extras, prices, and other document fields, then calls the same
authorized Offering write operation used by REST. Both transports use model
validation, expected revisions, and transactional replay storage.

Use a 16–128 character request key matching
`^[A-Za-z0-9][A-Za-z0-9._:-]{15,127}$`. Retry the exact input with the same key.
The tool reads the immutable requested base revision, so a retry after the draft
head advances can reproduce the original mutation and return its retained result.
A changed command with that key is rejected. A fresh command based on a stale
revision is rejected rather than overwriting newer work. The reply's `replayed`
flag describes this attempt; it is not part of the retained menu revision.

For inspection, omitted/zero `revision` means latest. If `offset` is positive,
an explicit revision is required. Continue using the returned revision and
`next_offset` so pages describe one immutable draft. Summaries deliberately omit
the full configuration of extras and variants; they are not enough to configure
a customer purchase. All prices are minor currency units.

`query` is a trimmed literal case-insensitive substring of item name or ID,
limited to 100 Unicode code points. No translation or fuzzy matching is implied.
Keep it unchanged while paging; `total_items` counts the whole menu and
`matching_items` counts the matches before the page limit. Empty results are an
empty array. An exact `item` ID, not a guessed display name, selects an item read.

`inspect_menu_item` returns all variants together and group summaries without
their options. `inspect_item_extras` returns complete selected groups (at most
three, with no duplicates), including zero-price options. It requires the item
read's positive revision and fails as a whole if any selected group is missing.
No partial success or silent omission is implied. This batches related details
without echoing every other dish; see the [design and remaining coverage](../../../../docs/MCP-TOOL-DESIGN.md).

Both detail levels preserve `default_option_ids`. Selected groups also return each
option's optional `variant_prices` map. Its values replace `price_minor` for those
size IDs; other sizes use the base surcharge. Zero is an explicit free override,
not a missing value. These are configuration prices, not a customer purchase quote.

### Dish prices and draft comparison

`update_item_prices` batches full size prices and extra surcharges for one dish.
Each change has an integer `price_minor` from 0 to 1,000,000,000 and either
`variant`, or `group` plus `option`, never both. Zero explicitly makes a size or
extra free. Select exact IDs from the detail tools. Duplicate targets, mixed
selectors, missing targets, fractional/negative amounts, and oversized batches
are rejected. The entire batch is one validated Offering write: it advances
one revision or changes nothing. Other fields and other dishes are preserved.
Its expected-base and retry-key behavior is the same as availability edits.

For a size-dependent extra surcharge, supply `group`, `option`, and
`extra_variant`, with `price_minor` as the replacement surcharge. The size must
exist on the dish. Omitting `extra_variant` changes only the base surcharge and
preserves existing overrides. Base and distinct size overrides may share one
atomic batch; repeating the same target is rejected. Removing an override is not
supported by this tool yet.

`review_menu_changes` compares immutable revisions, including edits made through
REST/UI. Each change gives an RFC 6901 JSON Pointer `path` and its `before` and/or
`after` leaf value. A missing side means an added/deleted field, not JSON null.
Dish-level changes also include `before_item` and/or `after_item` with ID and
name, so clients can identify the affected dishes without retrieving menu pages.
Empty arrays/objects are leaves; large nested additions are split into leaf
changes. Pointers are sorted lexically, and array indices belong to those
revisions, not durable item IDs. Reordering can therefore produce several
positional differences; this is an exact field comparison, not a semantic
reordering detector or a JSON Patch document. The two item contexts can differ
when dishes move. Object key order is ignored.

Continue with the same revision pair and `next_offset`; no changes yields
`changes: []`. Reviewing a new draft against its base shows only that edit;
reviewing against an earlier revision shows the accumulated changes. Review all
pages before judging the complete difference. No review call records human
approval, changes publication, or certifies readiness. The merchant workspace
remains the publication surface, with its existing explicit revision and
publication-generation checks.

### Meal offers

Use `inspect_meal_offers` for a store overview or filter summaries by an exact
main dish `item` ID. An unknown item fails rather than suggesting that it has no
offers. Summaries contain counts, not eligibility or add-on lists. To inspect an
offer, send its exact `offer` ID and positive immutable `revision`, without
`item`. The response contains the offer's name, translations, main dish IDs,
surcharge, add-on count, and at most 20 add-on choices with item/variant names
and draft availability. A positive offset also requires a revision. Keep all
selectors and that revision unchanged for continuation; unavailable choices are
reported, not silently removed. These are draft configuration reads, not quotes.

`save_meal_offer` edits one existing offer by default. Creating a new offer
requires `create: true` and an unused ID in the inspected revision. Send the
complete offer terms: `id`, `name`, `main_item_ids`, `surcharge_minor`, and
`addon_choices` containing `item_id`/`variant_id` pairs. Both lists contain 1–200
unique entries. Omitted `translations` preserve existing localized names; an
explicit object replaces them, including `{}` to clear them. Null is rejected.
The shared Food validator enforces supported locales, text/price constraints,
valid references, and the menu's maximum of 50 offers.

An offer adds its surcharge to each eligible main unit and includes one selected
add-on per unit; the add-on's standalone price is not also charged. Add-ons cannot
require modifiers, be customized separately, or recursively apply another deal.
An unavailable add-on may remain configured but cannot be ordered. Saving preserves
menu items and other offers, uses the shared Offering write, and has the same
revision/retry guarantees as dish edits. Review changes before separate publication
through the confirmed publication flow. Offer deletion is not exposed as a merchant tool yet.

## Errors and evidence

Application failures return MCP `isError: true` with a small JSON text object:
`code`, `next_action`, and, for revision conflicts, `current_revision`. Stable
codes reuse the common/Offering meanings: `invalid_request`, `forbidden`,
`not_found`, `idempotency_conflict`, `revision_conflict`, `unsupported_model`,
`invalid_offering`, and `unavailable`. `timeout` and `cancelled` identify an
interrupted attempt, not an assertion that its database mutation failed. Retry
the same request key to recover a retained result. SDK protocol/schema errors
remain JSON-RPC errors, not successful business results.

Authentication failures use HTTP 401 and a bearer challenge; local Host/Origin
denials use 403. Unsupported methods use 405 (`method_not_allowed`). These are
transport failures before tool execution. Business error text is safe guidance,
not raw database/exception text or an instruction to ignore merchant review.

Executed tests cover actual SQL tenant isolation, revisions, replay conflicts,
credential separation/expiry/revocation, menu-field preservation, and bounded
48-item menu inspection. SDK client and Inspector checks establish compatibility
with the tested release cohort, not production OAuth or model-driven task success.

## Menu setup additions

`inspect_menu_structure` returns ordered categories and counts, not full dishes.
`save_menu_item` changes only supplied fields of one exact dish; `create=true`
is required for new IDs. Supplied size/modifier arrays replace those complete
lists. `organize_menu_categories` replaces categories and explicit assignments
atomically. All three use Food's existing model; write results are private
draft revisions, not publication or customer price promises.

## Reviewed publication

`preview_menu_publication` compares an exact positive Food revision against the
current published revision, or an empty baseline before first publication.
Results contain at most 20 field changes, the current generation and suspension
flag, and a selection-binding token. Continuations must retain revision and
generation; changed generations require a new preview. Review every page.

`publish_menu_revision` requires offering read/write, that exact revision and
generation, the preview token, `confirmed=true`, and a retry key. Confirmation
must follow explicit merchant instruction. The digest is not authorization or
proof that a human reviewed the pages. The shared publication operation retains
audit/replay atomically and cannot clear provider suspension. Successful retries
return historical results; read again to learn current visibility.
