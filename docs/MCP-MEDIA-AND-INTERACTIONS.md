# MCP interactions for menus and ordering

Checked against primary MCP documents on 22 October 2026. These findings guide
the next tool and media slices; they do not mean the current seven merchant
tools implement Apps, customer baskets, elicitation, or Tasks.

## Keep commerce state explicit

The versioned MCP transport and our commerce model have different responsibilities.
A `basket_id` can identify a durable basket across requests, connections, and
clients. This is a useful **application design**, not a standardized MCP cart
schema. A basket still needs ownership, guest access, revision conflict checks,
expiry, idempotent mutations, and a fresh quote before purchase. A transport's
statelessness does not forbid state in PostgreSQL.

For the customer MCP, start with intentions: find a suitable restaurant, inspect
relevant dishes/options, assemble or change a basket, review a quote, place an
order, and check its status. Summaries should include prices and customization
requirements together when that saves round trips. Return images or detailed
groups on demand rather than loading the entire menu into every tool result.
Merchant setup/edit/review tools remain separately scoped.

## Images and interactive menus

[MCP tool results](https://modelcontextprotocol.io/specification/2026-07-28/server/tools),
Tool Result and Image Content, support images without an MCP App. Icons describe
tools or servers; they are not a substitute for dish photos. Actual media support
depends on the client, so we will not promise SVG/WebP rendering everywhere.
Default to ordinary JPEG food assets and retain useful text/structured results.

[MCP Apps](https://modelcontextprotocol.io/extensions/apps/overview), How it works,
adds interactive HTML through a tool-linked `ui://` resource in a host-controlled
sandboxed iframe. It could display menu cards, customization, and a basket with
less conversational overhead. It is not arbitrary remote HTML that every client
must render. Test a supporting host, its sandbox/CSP, upload origins, and tool
authorization; retain a text-based ordering path. UI state must not become a
second authority for prices or purchase decisions.

The [media foundation](MEDIA.md) prepares direct S3-compatible uploads and demo
photos. It does not yet expose images through merchant/customer MCP results.

## Ask for missing choices without holding a purchase open

[Multi round-trip requests](https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr),
High level flow and InputRequiredResult, lets a server request more input and
the client retry with responses and opaque request state. It does not require a
paused in-memory execution or an open database transaction. Missing required
sizes or option choices are good candidates; an allergy question cannot turn an
unverified ingredient list into a food-safety assurance.

Prefer returning required options before checkout when possible. On resumption,
recheck authority, basket revision, availability, and current quoted terms. Do
not treat elicitation as payment authentication or an agent's claimed authority.
Keep structured missing-choice results usable for clients without rich prompts.

## Cache the appropriate things

The [caching specification](https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/caching),
Cacheable Results, enumerates discovery/list operations and `resources/read`.
It does **not** make a `tools/call` result such as our `inspect_menu` automatically
cacheable through `ttlMs`. A revision-addressed menu resource is a possible
future fit; private drafts must remain private to their authorization context.
TTL is a freshness hint, not a price guarantee or a mandatory polling interval.
No source reviewed establishes a universal 10 ms latency outcome.

## Separate a task from the order it created

The [Tasks extension](https://modelcontextprotocol.io/extensions/tasks/overview),
Lifecycle and Client Interaction, describes durable handles for long-running tool
work. A future large menu import may benefit. A successful order-placement call
can finish while the restaurant is still preparing the meal: task completion is
not payment success, readiness, pickup, or delivery. Read order state through an
authorized API/tool, with the existing SSE path for the web application.

## Next implementation order

1. Add the authorized media-initiation/completion contract and optional owned
   media references; display the three demo photos through the real offering path.
2. Continue merchant setup/edit/review intentions, then design customer tools
   around explicit basket/quote/order records and compact task-relevant results.
3. Exercise the full workflow in Inspector and a real model/client. Add MCP Apps
   as progressive enhancement in a supporting host, with the same authorization.
4. Build the admin workspace, including APIs and MCPs, after notifying the founder
   and receiving their offered visual references. That redesign has not started.

Do not make a dependency on every new MCP extension a prerequisite for a useful
menu or ordinary ordering. Check each feature against the pinned SDK and actual
client before claiming support; the existing transport tests do not establish it.
