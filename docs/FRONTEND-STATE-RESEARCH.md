# Frontend state ownership

24 October 2026. This implementation note explains the first XState/TanStack
integration, not a proposal to rewrite every screen. The immediate problem is
keeping an order receipt current without polling, duplicated order records, or
private data surviving a change of customer.

## Decision and version boundary

Use stable XState 5 for lifecycle coordination, and
TanStack Svelte Query 6 for fetched server data. Registry inspection returned
`xstate` 5.33.3, `@xstate/svelte` 5.0.1, and `@tanstack/svelte-query` 6.3.5.
The latter requires Svelte 5.25 or later; this app uses 5.57.2. These are version
observations, not evidence that our integration works. Pin the selected versions
and verify their installed types and behavior before adoption.

XState 6 is an alpha and is not the baseline. Svelte Query 6 is a released
Svelte-specific adapter over Query Core 5; do not infer its API from React's
package major or copy Svelte Query 5's store syntax. Version 6 takes reactive
option getters and exposes results directly through runes.

Sources: [XState version guidance](https://dev.stately.ai/docs/xstate/v6/faq),
[Svelte Query overview](https://tanstack.com/query/latest/docs/framework/svelte/overview),
[Svelte migration guide](https://tanstack.com/query/latest/docs/framework/svelte/migrate-from-v5-to-v6),
and the [tagged adapter source](https://raw.githubusercontent.com/TanStack/query/@tanstack/svelte-query@6.3.5/packages/svelte-query/src/createQuery.ts).

The first connection coordinator can remain a plain TypeScript actor created
inside the component effect. It does not need `@xstate/svelte` merely to report
connection status. The Svelte adapter is appropriate when a component consumes
machine snapshots directly; avoid adding it without that use case.

## One owner for each kind of state

The backend owns order transitions, payment, agreed terms, and durable work.
TanStack owns the browser's fetched order snapshot and its loading/error state.
XState owns the connection lifecycle: connecting, connected, reconnecting,
paused, and stopped. It must not maintain a second copy of the order or replace
the backend's recovery mechanisms.

An SSE message is a freshness hint, not an authoritative order record. It causes
an authorized JSON read. Reconnection also causes a read because notifications
can be lost. Reads remain usable by ordinary API clients independently of SSE.
No timer-based order polling or mandatory broker is introduced.

XState's invoked actors are stopped when their owning state exits. Async work
belongs in an actor rather than an async action; stopping a promise actor alone
does not undo its external work. Pass its abort signal to cancellable I/O.
See [invocation](https://stately.ai/docs/invoke),
[promise actors](https://stately.ai/docs/promise-actors), and the
[pinned promise implementation](https://raw.githubusercontent.com/statelyai/xstate/xstate@5.33.3/packages/core/src/actors/promise.ts).

## Cache and request boundaries

Private query clients must be scoped to the page/request, never a server module
singleton. Seed the receipt from the existing server load and keep credentials
in HttpOnly cookies. A same-origin server endpoint performs the authenticated
read; neither query keys nor client state may contain bearer tokens. Destroy
the page's cache on navigation, and react to changed route identity.

Explicitly disable automatic retries and focus/reconnect refetch in this first
slice: the SSE coordinator owns reconnection and invalidation. Default Query
behavior otherwise includes stale-on-mount reads, focus refetch, and retries.
Consume the query's AbortSignal and test stale responses, teardown, and a hint
arriving during a read. Do not automatically retry a mutation or treat a failed
refresh as permission to display a successful action.

Sources: [Svelte SSR guidance](https://tanstack.com/query/latest/docs/framework/svelte/ssr),
[shared Query defaults](https://tanstack.com/query/latest/docs/framework/react/guides/important-defaults),
[query cancellation](https://tanstack.com/query/latest/docs/framework/react/guides/query-cancellation),
and [SvelteKit state management](https://svelte.dev/docs/kit/state-management).

## Adoption checks

Before calling the slice complete, test initial SSR data, an SSE-triggered
refresh, reconnection, hidden-tab suspension, manual refresh, cancellation on
teardown, and isolation across route/account changes. Keep accessible native
forms and the no-JavaScript receipt. Run the existing browser regressions,
Svelte checks, unit tests, and relevant mutation tests. Record actual results in
the development status; documentation research alone proves none of these.

Before implementation, the tagged XState 5
[callback actor source](https://raw.githubusercontent.com/statelyai/xstate/xstate@5.33.3/packages/core/src/actors/callback.ts)
was also inspected: stopping the actor invokes its cleanup function and prevents
late callback sends. This supports state-owned EventSource cleanup, but our tests
must still prove cleanup and cancellation across pause, retry, and navigation.

## First implementation slice

The shared `live-updates.ts` coordinator now expresses connection and serialized
read lifetimes with XState. Pausing or disposing it stops its EventSource actor,
cancels connection retry, and aborts the read actor. A burst retains one trailing
read; an open/reopen requests fresh state even if its notification was lost.
Existing merchant/menu pages still use their SvelteKit load invalidation. That
fallback does not acquire cancellable I/O merely because the coordinator has an
AbortSignal; the new receipt adapter explicitly consumes it.

`CustomerReceipt.svelte` owns a TanStack client, seeded from server load. Its
query remains disabled for automatic fetching: SSE/manual hints explicitly
refetch it. Rendering and payment/cancellation revision fields use that same
query snapshot. The parent keys the component by the loaded order object so
navigation or a native form/server reload discards the old cache. Destroying the
component clears the client; no browser storage or server singleton retains it.

The same-origin `/stores/{slug}/orders/{order}/state` endpoint uses exactly the
existing receipt cookie lookup and authorized backend read. It forbids caching
and returns no bearer. Browser cancellation propagates through TanStack's query
signal and the server fetch, alongside the existing twelve-second timeout.
Native payment/cancellation forms and SSR/no-JavaScript rendering remain intact.

Focused tests cover queued hints, native reconnect, hidden-page/disposal cleanup,
construction/read failure, manual refresh, initial data without a read, isolated
clients, error retention, and cancellation without accepting a late response.
The full gate passed 101 frontend tests and zero Svelte diagnostics. The XState
coordinator's focused mutation run scored 97.37%; see
[mutation scope and limits](MUTATION-TESTING.md). End-to-end evidence is recorded
separately in [development status](STATUS.md), not inferred from these unit tests.
There is no registered-account switch UI yet: client isolation tests do not
establish correctness of a future OAuth login/logout transition. Include that
transition's cache teardown when implementing production account sessions.
