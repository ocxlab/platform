# Live order status

The customer receipt, merchant order detail, and merchant inbox receive SSE change
hints. They still read current records through the ordinary authorized JSON APIs.
This separates a responsive screen from reliable business state: missing a hint
does not lose an order, and receiving one does not establish that payment succeeded.

## Reads and commands remain ordinary APIs

Agents and other applications can GET an order, including its revision, separate
acceptance/payment/fulfilment states, and ordered history, without opening a stream.
Merchant inbox reads remain paginated. No browser or universal customer account is
required for the API; callers need the appropriate merchant or store-bound guest
credential. A future MCP adapter can expose the same reads and commands rather
than inventing another lifecycle. This batch does not implement an MCP server.

Placement, decisions, cancellation, and development-payment/fulfilment commands
return the result of their bounded database transaction. They do not wait for the
restaurant to finish cooking or a future real payment adapter to settle money.
Existing expected revisions and idempotency keys still protect mutations. A client
retry reads/replays through the same authorized operations; a hint never submits
a command on its behalf.

When real adapters add long-running work, return a durable operation reference
and a pending state once that work is recorded; use `202 Accepted` when processing
itself remains pending, with an authorized status URL. Do not relabel today's
committed `201` responses as `202`, or introduce a universal async wrapper for
operations that already complete in a short transaction. Real-adapter operation
contracts and workers remain future work.

## Stream contract

The modular OpenAPI entry defines these bearer-authenticated endpoints:

| Audience | Endpoint suffix beneath its merchant/store scope | Required access |
| --- | --- | --- |
| Merchant | `/food/order-updates` | `order:read` for the merchant and existing store |
| Guest | `/food/order-updates/{order_id}` | `order:read` and ownership of that order in that store |

They return UTF-8 `text/event-stream`. The initial connection and matching changes
emit `event: invalidate` with `data: {"resource":"food_orders"}`. Clients re-read
the authorized resource. Hints contain no selection, amount, identity, or payment
detail. Notifications can be coalesced; there is no history-replay cursor or
`Last-Event-ID` guarantee. Initial hints on reconnect close the gap by requesting
a fresh read. The durable JSON history, not the stream, supplies past events.

The web uses same-origin `/updates` routes because native browser EventSource
cannot attach the backend bearer header. Those routes resolve existing HttpOnly
sessions and forward to a fixed backend origin/path. They never place credentials
in URLs or expose them to browser JavaScript. Direct API clients can supply their
own Authorization header and consume the SSE response body. Generated JSON
convenience parsers are not incremental SSE parsers.

## Delivery and resource boundaries

Migration 11 adds an event-insert trigger using PostgreSQL `NOTIFY`. Notifications
are delivered after commit and disappear on rollback. Their database-wide payload
contains merchant, store, and order UUIDs only; it is not an encrypted or private
message bus. Only trusted service database roles may connect to this database.
One dedicated direct PostgreSQL session per API process executes `LISTEN`; it
must not use a transaction-pooling proxy. Multiple API processes connected to the
same database each receive hints. Separate providers do not share this channel.

Each process filters subscriptions by merchant/store and, for guests, exact order.
Hints are buffered/coalesced without blocking a transaction. A disconnected
listener closes existing streams and refuses new subscriptions until reconnected.
Its bounded connection attempts retry after one second. Limits are 1,000 active
streams per process and eight per grant; these are initial safeguards, not load
test results or a substitute for deployment-level connection/rate limits.

Streams last at most two minutes. Ten-second heartbeat comments keep the transport
active and recheck grant validity; these checks do not poll order progress.
Every actual hint also rechecks access to the resource before emission. Revocation
ends a live stream at the next check, not retroactively at the previous hint.
Short write deadlines bound stalled writes. No transaction or pooled query
connection stays held for the life of a browser stream.

The UI closes its source while hidden, reconnects when visible, and disposes it
on navigation. A fresh connection requests current data. Only one state refresh
runs at a time, with a trailing refresh if hints arrived during that read. There
is no periodic order-state polling or automatic mutation. Manual refresh and
no-JavaScript navigation remain available. Hidden-tab/background order alerts
need a separate, explicitly designed notification feature.

## Verification and deployment follow-up

Unit checks cover scope filtering, coalescing, limits, disposal, and frontend
connection/refresh behavior. PostgreSQL/HTTP tests exercise commit versus rollback,
guest ownership, credential audiences, reconnect resynchronization, normal JSON
reads, and live grant revocation. Browser acceptance exercises updates between
separate merchant/customer sessions without manual refresh and the shared timeline.
Final executed results are recorded in [STATUS.md](STATUS.md).

Before public deployment, verify proxy buffering, idle/maximum request durations,
HTTP connection limits, listener health/metrics, authenticated connection quotas,
and load under the chosen hosting model. This is not cloud-failover or a
performance/cost benchmark. A larger deployment can replace hint delivery without
changing the authoritative read/command contract.

Primary references consulted 22 October 2026, Europe/Berlin:

- [WHATWG server-sent events](https://html.spec.whatwg.org/multipage/server-sent-events.html),
  EventSource interface, processing/reconnection model, and event-stream format.
- [PostgreSQL NOTIFY](https://www.postgresql.org/docs/current/sql-notify.html),
  transaction commit/rollback delivery and database-wide notification visibility.
- [HTTP Semantics, 202 Accepted](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.3),
  acceptance without completed processing; this is distinct from a completed
  database command or guaranteed external outcome.
