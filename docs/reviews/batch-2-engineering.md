# Batch 2 engineering review — order operations

Reviewed 21 October 2026, Europe/Berlin (UTC+02:00), against the uncommitted
working tree relative to `6297c78`. This is a static engineering and security
review. The main implementation agent owned the active PostgreSQL/browser
runners, so this review did not run shared Docker test tasks.

## Scope reviewed

- merchant order inbox/read and accept/reject;
- guest cancellation while payment and fulfilment remain `not_started`;
- separate, read-only provider order inspection;
- immutable placement terms plus a mutable progress row, ordered event history,
  and scoped durable replay;
- modular OpenAPI sources and generated Go/TypeScript surface;
- local development credential bootstrap only.

Payment, preparation, auto-acceptance, delivery, and network runtime behavior
are intentionally outside this review.

## Findings

| Severity | Finding | Exact location | Bounded correction |
| --- | --- | --- | --- |
| Medium | The merchant “inbox” is ordered and paged by random UUID, rather than by when a restaurant needs to see the order. A later actionable order can therefore appear arbitrarily far from the first page. The contract explicitly documents this behavior, but documenting it does not make it useful for restaurant operations. | `services/provider/internal/persistence/order_operations.go:157-184`; `contracts/openapi/v1/paths/merchant-food-orders.yaml:7-10` | Keep the compact UUID cursor if desired, but resolve its `(created_at, id)` tuple server-side and return newest-first with `ORDER BY created_at DESC, id DESC`; add the matching tenant/store index and a continuation test which proves chronological, non-duplicating pages. Update the path description. This is a bounded inbox correction, not filtering, search, or a queue feature. |

## Resolved during this review

The early revision-conflict finding has been corrected before this report:

- `ChangeFoodOrder` now returns `OrderRevisionConflict` with the locked current
  revision (`order_operations.go:13-15, 96-98`).
- HTTP maps it to `order_revision_conflict` with
  `current_order_revision` (`server.go:247-252`), and the reusable Problem
  schema declares that field.
- The PostgreSQL integration test asserts both the error code and the current
  revision (`order_operations_test.go:48-56`).

That makes a stale merchant or guest action recoverable without guessing which
version to submit. The separate field avoids conflating a Food order progress
revision with an Offering draft revision.

## Boundaries that are sound in the reviewed slice

- The immutable `food_orders.body` is not updated. Operational state is held in
  `food_order_operations`, locked with `FOR UPDATE`; ordinary reads use a
  compatible share lock, then load the append-only event history. The action,
  event, and replay result use the same transaction.
- The transition guard keeps the bounded slice honest: accepting/rejecting or
  guest cancellation requires an open order with both payment and fulfilment
  still `not_started`. Acceptance does not claim payment or start preparation.
- Guest cancellation is scoped both by the authenticated store grant and the
  order's `guest_id`; another guest receives no usable order state. Merchant,
  guest, and provider audiences are distinct, and provider inspection remains
  read-only with the `order:inspect` scope.
- RLS is enabled and forced on the new progress table. Runtime grants allow only
  the state columns needed for transitions, and event/replay tables remain
  append-only to the application role.
- Replay fingerprints include operation/resource/body and replay rows are
  principal-scoped. The tests cover same-key replay, conflicting keys,
  acceptance-versus-cancellation serialization, rollback when either event or
  replay persistence fails, restart behavior, and cross-tenant RLS visibility.
- `deploy/local/enable-order-scopes.sql` is explicitly restricted to the local
  `ocp_dev` database and development role. It is not a migration and does not
  silently widen production or test grants. New development credentials include
  the required scopes in `cmd/provision`.

## Bounded verdict

**Conditional acceptance after the chronological inbox correction and the
owner's declared full check.** I found no separate authorization, immutability,
replay, or concurrent-transition blocker in this bounded slice. The full
`task check` and browser run remain the execution evidence; a passing build
does not add payment, kitchen, auto-acceptance, or production-auth claims.

## Final static disposition — 21 October 2026

The chronological-inbox finding is resolved in the modular source:

- `ListFoodOrders` now uses an opaque, bounded base64url cursor that contains
  the final `(created_at, id)` position, parses it before opening the tenant
  transaction, and seeks with the matching descending tuple
  (`order_operations.go:157-186`).
- Migration `00004` adds the matching `(merchant_id, store_id, created_at DESC,
  id DESC)` index. The source OpenAPI contract describes newest-first ordering,
  the opaque cursor, and invalid-cursor `400` behavior.
- The integration coverage now exercises a nonempty continuation page, and the
  unit coverage includes valid cursor round trips and invalid inputs.

The cursor is not treated as authority and the live-page qualification remains
appropriate: a merchant refreshes the first page to see new arrivals.

**Static acceptance, pending mechanical verification:** no remaining source-level
engineering blocker is identified for Batch 2. At this exact inspection point,
the generated TypeScript declaration still contained the superseded UUID-order
description and the changed Go source still needed `gofmt`; these are expected
mechanical follow-ups, not design defects. The acceptance becomes final only
after the owner's regeneration/formatting and declared `task check` plus browser
verification show them clean.
