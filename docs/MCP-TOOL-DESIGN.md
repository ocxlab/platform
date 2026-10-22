# Task-oriented MCP tools

Design checkpoint: 22 October 2026, Europe/Berlin (UTC+02:00).
This note separates the implemented merchant read path from the remaining
merchant and customer tool design. Neither three tools nor a particular tool
count is the objective: owners should complete useful tasks safely without
loading unrelated information or making avoidable calls.

## Information when the task needs it

An owner asking about the price of a large Döner needs the matching dish and
its sizes, not every pizza topping in the store. An owner checking extra cheese
also needs the relevant extras and their surcharges. That difference determines
what each response contains.

The current merchant endpoint supports this progression:

| Task | Tool | Information returned |
| --- | --- | --- |
| Find the relevant managed store | `find_my_stores` | At most 20 names, handles, draft revisions, and item counts |
| Find a dish or scan the menu | `inspect_menu` | Optional name/ID substring search; at most 20 summaries with variant/extra-group counts |
| Understand one dish | `inspect_menu_item` | All sizes and full prices; availability; extras-group names, selection limits, and option counts |
| Inspect relevant extras | `inspect_item_extras` | Options and surcharges for one to three selected groups of that dish |
| Mark a dish available or unavailable | `set_item_availability` | A focused private-draft change with a revision and replay flag |
| Correct size prices or extra surcharges | `update_item_prices` | One atomic private-draft edit for up to 20 distinct prices on a dish, with base/saved revisions and replay flag |
| Review the difference before publishing | `review_menu_changes` | Up to 20 exact field changes per page between explicit retained revisions, with before/after values |

This is progressive disclosure of **information**, not progressive loading of
tool definitions. A client may go straight to an item when it already has its
ID and revision. The sequence is not a required wizard. Inspecting availability
does not require loading extras, and all sizes arrive together rather than one
call per size. Related extras groups can be requested together. An item with
no extras needs no extras call.

Group summaries preserve required/maximum choice counts. Extras preserve free
options explicitly as zero, and prices are integer minor currency units. Search
returns every matching summary up to its page limit, never silently choosing
between similarly named dishes. It is literal, case-insensitive name/ID matching,
not fuzzy search, translation, transliteration, or recommendation. Keep the same
query and explicit revision while following `next_offset`. Results stay in
document order. An empty search is `items: []`, not a failed read.

An item read accepts the returned draft revision; extras require a positive
revision. This keeps the details consistent even if a different client saves a
new draft between calls. The 200-item model bound limits the source document;
each item has at most 30 variants and 20 groups, with at most 30 options per
group. The extras batch therefore returns at most 90 options. This is an initial
bound to evaluate, not a claim of optimal granularity for every menu.

Price changes batch related size/extra corrections without replacing an entire
menu in tool arguments. Missing or duplicate targets reject the whole edit;
zero-priced extras remain explicit. Draft review can compare UI and MCP edits
alike, so agents do not need to remember what they previously changed. Its
JSON Pointer locations describe exact revision-local fields, not durable item
identity. Reordered arrays can generate positional differences. The current
review is mechanical and paginated; it is not human approval, a semantic
change summary, or a publication capability. Publishing still happens in the
merchant workspace.

## Balance context size against calls

Optimize complete intentions, not isolated payloads. Include tool definitions,
arguments, both structured/text results a client may retain, request latency,
and repeated reads after errors. A short response that forces five extra calls
can be worse than one focused, moderately detailed response.

Use the 48-item acceptance menu for a known-store scenario: find one Döner,
read its sizes and group summaries, then read the sauce and extras together.
Record request count and serialized result bytes separately from source-menu
size. These are reproducible diagnostics, **not model token counts**. Include
store discovery when the agent does not already know the store. Record schema
cost separately so it is not hidden by reporting only small result bodies.

No language model or tokenizer is required for these structural checks.
Later assistant evaluations must compare successful task completion, actual
token use, latency, ambiguity handling, and permission failures. Do not infer
task quality or faster execution from byte counts alone. If common tasks still
require unnecessary calls, add a narrowly defined detail option or focused batch
operation; do not introduce an unrestricted API dispatcher.

Executed diagnostic on 22 October 2026: that known-store, three-call fixture
returned 743 bytes for search, 1,141 for the item, and 1,545 for both extras
groups: **3,429 result bytes** including the text/structured copies. The source
menu document was 21,393 bytes; the five-tool discovery result was 4,314 bytes.
These count JSON-RPC result objects, not complete HTTP exchanges. The source
document is not itself an equivalent full-menu MCP result, and neither figure
is a token or latency measurement. The fixture uses a prepared query and item,
not an agent demonstrating it can choose the right calls.

## Coverage still to build

The following intentions guide the next increments; the names are not promises
of existing tools. Add their shared application operations first where missing.

| Audience and area | Intent | Design boundary |
| --- | --- | --- |
| Merchant commerce | Create a menu from described products or imported material | Bounded validated drafts; treat imported text as data; report missing prices/choices rather than invent them |
| Merchant commerce | Correct a dish, size, or extra | Focused changes, reviewed base revision, preserved unrelated fields, and replay-safe writes |
| Merchant commerce | Review and publish changes | Compact meaningful diff and warnings; publish exactly the reviewed revision with explicit authority/approval, not the latest unreviewed draft |
| Merchant commerce | Manage incoming orders and availability policy | Bounded named inbox, current state, permitted actions, and explicit merchant decisions; menu rights do not imply order rights |
| Merchant account | Manage store/business details and delegated access | Account scope independent of commerce scope; stronger checks for sensitive access changes |
| Customer commerce | Find stores and suitable dishes | Public published information, search summaries, details only as needed; no merchant drafts |
| Customer commerce | Configure, quote, and place an order | Server-validated combinations and current quote; material prices/terms visible before commitment; replay-safe placement |
| Customer commerce | Pay, check progress, or cancel | Own-purchase authority; provider-hosted payment interaction where needed; no card secrets in model arguments/results |
| Customer account | Manage addresses, preferences, and delegation | Private customer account authority; a browsing session is not an account login |

Keep merchant and customer endpoints separate and keep provider administration
outside both. Guest ownership capabilities must not become broad merchant or
customer-account authority. New tools reuse the authorized application operations,
not public network calls to our own REST endpoints. Tool read/write annotations
help clients present actions but do not enforce ownership, scopes, or approval.

## Discovery and evidence

Keep `tools/list` deterministic for the same granted authority. Do not make the
tool set change merely because an agent inspected a dish. Filter by authority
and enforce policy again in every call. MCP's tool guidance covers explicit
schemas, structured results, annotations, and safe error handling; the choices
above are our task-oriented application design, not a mandated MCP tool catalog.

Primary references checked on 22 October 2026:

- [MCP tools, 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/server/tools), Capabilities, Listing Tools, Tool Result, and Error Handling.
- [Understanding MCP servers](https://modelcontextprotocol.io/docs/2026-07-28/learn/server-concepts), Tools and Resources: tools are explicit operations; resources are a separate context mechanism, not a prerequisite to this read path.

See [current contract](../contracts/mcp/v1/merchant/README.md) for implemented
inputs and errors, and [runtime setup](MCP.md) for local authentication and
verification limits. Public OAuth, customer tools, broader menu editing,
publication, model-driven evaluation, and the MCPs admin area remain unfinished.
