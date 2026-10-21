# Development payments protocol review

**Review date:** 21 October 2026, Europe/Berlin (UTC+02:00)
**Snapshot:** uncommitted development-payment, readable-reference, and related
contract changes after the follow-up corrections.
**Method:** static source and contract review only. This reviewer did not run
Docker, PostgreSQL, generation, mutation, or browser tasks. The implementation
run reports a passing full `task check`; its browser verification was still in
progress at this review point.

## What is sound

- The development endpoint has a distinct, guest-only path and is bound to the
  placing guest's merchant, store, and order through the existing guest audience,
  scope, and order lookup. It is disabled by default in `NewHandler` and enabled
  only by the explicit local/test option; the local Docker composition makes that
  choice visible. Its response semantics state plainly that it moves no real
  money and does not trigger preparation.
- Payment state is held in `payment.development_operations`, not embedded as a
  mutable rewrite of immutable order terms. Payment simulation, its event,
  current order revision, and replay result share the mutation transaction. The
  order progress lock is acquired before financial state is read, avoiding a
  response that combines new order progress with an earlier payment snapshot.
- Expected revision, idempotency, maximum simulated attempts, closed-order
  rejection, and the paid-cancellation/refund simulation form a coherent bounded
  lifecycle. A successful historical replay intentionally differs from a fresh
  current-order read. No completed payment claims kitchen or fulfilment progress.
- The provider-local `store-N` and `O-N` references are display aids only;
  UUID path identifiers and authorization remain authoritative. Optional fields
  preserve old immutable placement/replay bodies through the migration.

## Follow-up findings and verification

| Finding | Static disposition |
| --- | --- |
| A generic `FoodOrder.payment_provider` had been limited to `development`, which would make another adapter invalid under the same Food contract. | **Resolved.** `contracts/openapi/v1/domains/food/order/v1/schema.yaml` now defines a bounded opaque provider identifier and documents that only `development` is currently emitted. The current payment-state model is therefore not presented as an exclusive production-provider vocabulary. |
| Guest cancellation bypassed the general payment-state guard but only actually compensated the development adapter, creating a future unsafe path for another paid provider. | **Resolved.** `services/provider/internal/persistence/order_operations.go` permits a non-`not_started` cancellation only for `development`; `ChangeDevelopmentPayment` likewise rejects a non-development provider. A future adapter must provide its own authorized compensation/cancellation behavior rather than inheriting simulated refund semantics. |

## Remaining bounded gates

- This is a local development simulator, not a PSP adapter, webhook contract,
  payment-provider roaming implementation, or production payment service.
  Real adapters require their own provider identity, authorization, asynchronous
  outcome/reconciliation, compensation, and jurisdictional handling before being
  enabled.
- The Food history maximum is adequate for the stated simulator bound (placement,
  decision, up to ten simulated attempts, and cancellation), but individual-order
  history needs paging or a revised retention contract before unbounded payment or
  fulfilment event sources are added.

## Bounded verdict

**Static protocol review accepted, contingent on the running browser verification
also passing.** The slice maintains a clear boundary between a simulated local
payment adapter and the Food order lifecycle, and it does not make a real-money
or production-PSP claim.
