# Development payments and readable references engineering review

Reviewed 21 October 2026, Europe/Berlin (UTC+02:00), against the uncommitted
working tree after `e904d1f`. This is a static engineering and security review.
The main implementation agent owned the active PostgreSQL, browser, and mutation
runners, so this review did not run shared Docker tasks.

## Scope reviewed

- durable, explicitly development-only payment simulation for an accepted food
  pickup order;
- payment/order locking, transition rules, cancellation/refund simulation, and
  scoped idempotent replay;
- payment-schema RLS, runtime-role restrictions, and OpenAPI access boundary;
- readable store handles and order references while UUIDs remain authoritative;
- customer receipt refresh/actions and the bounded mutation-test setup.

Real payment processors, webhooks, fulfilment preparation, provider roaming,
and federation are intentionally not part of this review.

## Findings

No remaining material blocker was identified in the reviewed source.

One security issue found during this review was corrected before this final
disposition: the restricted runtime-role check originally omitted the new
`payment` schema. `CheckRuntimeRole` now rejects ownership of payment tables or
the schema itself (`services/provider/internal/persistence/database.go:146-149`),
and the integration fixture transfers both objects to `ocp_app` as negative
controls before restoring them (`services/provider/internal/integration/foundation_test.go:440-467`).

## Sound boundaries

- The simulation is opt-in at server construction and disabled by default.
  `cmd/api` enables it only with `OCP_DEVELOPMENT_PAYMENTS=true` and emits a
  warning that no real money moves (`cmd/api/main.go:49-53`). The endpoint
  itself returns the registered 503 problem before any mutation when disabled
  (`internal/httpapi/development_payments.go:12-16`).
- A guest may act only on its own store-scoped guest order. The operation requires
  a guest credential and the existing `order:create` capability, while the
  server checks credential audience, merchant, store, and scope before calling
  the handler (`internal/httpapi/server.go:57-103, 134-179`).
- Food retains authority over acceptance and fulfilment. The payment module owns
  only a compact financial-operation state machine; a development action is
  allowed solely for an accepted, open, unprepared order
  (`persistence/development_payments.go:42-70`). It neither claims a processor
  result nor starts preparation.
- The mutable order-operation row is locked before payment state is read. This
  prevents a waiting request from continuing with an obsolete join snapshot;
  payment completion, cancellation, and merchant decisions therefore serialize
  on one order (`persistence/order_operations.go:60-115`,
  `persistence/development_payments.go:42-70`). State, event, and durable replay
  result remain in the same transaction.
- A simulated paid cancellation changes the operation to `refunded` and records
  `cancelled_refunded` atomically. An awaiting or failed/not-started simulated
  attempt cannot be turned into a paid closed order
  (`persistence/order_operations.go:141-186`,
  `payment/development.go:39-48`).
- `payment.development_operations` is forced-RLS with the transaction-local
  merchant context and grants only the application columns needed for this
  bounded state transition (`migrations/00007_development_payments.sql:4-21`).
  The amount and currency come from immutable agreed order terms, not from the
  request (`development_payments.go:22-25`; contract schema
  `schemas/payments/development.yaml`).
- Store handles and order references are presentation identifiers only. They are
  non-resetting identity-sequence values, while API paths and authorization still
  use UUIDs (`migrations/00006_readable_references.sql:2-6`,
  `persistence/references.go`). Historical placement/replay bodies intentionally
  may omit the subsequently assigned display value; current reads populate it.
- The web receipt sends its guest bearer only server-side, exposes development
  controls only when the local flag is set, and has a real full-page refresh
  link for later merchant/payment status (`apps/web/src/routes/stores/[merchant]/[store]/orders/[order]/+page.server.ts:7-40`,
  `+page.svelte:15-34`).

## Limits and follow-up, not blockers

- `development` is a local simulation adapter. Its guest-controlled `succeed`
  action is correct only because it has no external financial effect. A real
  adapter must use a provider-confirmed outcome and durable reconciliation;
  it must not reuse that control surface.
- The initial references are database-local, which is equivalent to
  provider-local while this implementation has one provider database. Future
  independent-provider display/reference conventions need an explicit namespace
  or issuer rule; UUIDs already remain the durable authority.
- The environment switch is suitable for the declared local workflow and is
  default-off. Deployment policy should prevent setting it in any production
  environment when such an environment exists.
- The mutation configuration improves regression sensitivity for the selected
  TypeScript and Go rule modules. It is not a substitute for the owner’s full
  integration/browser verification of persistence, generated artifacts, and
  Docker wiring.

## Bounded verdict

**Static acceptance, pending the owner’s already-running mechanical checks.**
The reviewed implementation keeps the development-payment boundary explicit,
persists its state rather than fabricating a response, and preserves tenant
scope, immutable terms, replay behavior, and serialized order transitions. It
does not make a production-payment, webhook, kitchen, or federation claim.
