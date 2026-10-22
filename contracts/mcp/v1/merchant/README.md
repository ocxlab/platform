# Merchant tool contract

The first merchant MCP slice helps owners find a managed store, inspect its menu,
and mark an item unavailable in a private draft. It is an intention-oriented
interface, not an export of every REST operation. Publication remains a separate
merchant workspace action, so an agent's draft edit cannot change the live menu.

The endpoint is `/mcp/merchant`. Its local implementation uses the released Go
SDK's Streamable HTTP transport. This developer slice is not a public OAuth
resource server. See [setup and boundaries](../../../../docs/MCP.md).

## Tools and permissions

| Tool | Required merchant scopes | Input | Compact result |
| --- | --- | --- | --- |
| `find_my_stores` | `store:read` | Optional `after` continuation | Up to 20 names, handles, draft revisions, and item counts; optional `next_after` |
| `inspect_menu` | `offering:read` | `store` handle; optional literal `query`, `revision`, and `offset` | Up to 20 matching item summaries, currency, revision, whole-menu `total_items`, `matching_items`, and optional `next_offset` |
| `inspect_menu_item` | `offering:read` | `store`, exact `item`; optional `revision` | One dish's availability, all variant IDs/names/prices, and extras-group IDs/names/limits/counts, not option lists |
| `inspect_item_extras` | `offering:read` | `store`, exact `item`, positive `revision`, one to three distinct `groups` IDs | The selected groups with choice limits and all option IDs/names/surcharges, preserving requested group order |
| `set_item_availability` | `offering:read`, `offering:write` | `store`, exact `item` ID, boolean `available`, positive `expected_revision`, `request_key` | Saved draft revision, item availability, replay flag, and next action; no full menu echo |

Scopes are checked in discovery and invocation. Every HTTP request authenticates
the grant again, including expiry and revocation. Store handles select records;
they grant no authority. Merchant ownership and PostgreSQL tenant policies remain
in force. Provider-admin and guest credentials are not merchant credentials.

The schema-authoring source is the typed inputs in
[`agenttools`](../../../../services/provider/internal/agenttools/merchant.go)
and its [menu detail inputs](../../../../services/provider/internal/agenttools/menu_details.go).
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
