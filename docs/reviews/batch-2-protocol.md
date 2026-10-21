# Batch 2 protocol review

**Review date:** 21 October 2026, Europe/Berlin (UTC+02:00)

**Snapshot:** uncommitted diff from `6297c78`, reviewed statically.
**Scope:** merchant order inbox/read and accept/reject, guest cancellation,
provider inspection, operation revisions/history/replay, and the modular OpenAPI
surface. This review did not run Docker, database, or browser tasks because those
checks are owned by the implementation run already in progress.

## What is sound

- The new mutable `food_order_operations` record deliberately separates current
  merchant decision/resolution from the immutable placement body. The read path
  overlays those current facts while retaining the original terms; placement
  replays may therefore remain historically byte-identical.
- A guest credential is still constrained independently by audience, merchant,
  store, and guest-owned order. Guest cancellation rechecks the grant inside the
  mutation/replay transaction. Merchant and provider credentials cannot be used
  as guest credentials, and provider inspection is read-only at this API surface.
- Decision/cancellation operations use an expected operational revision, scoped
  idempotency, a row lock on the mutable operation record, and an append-only
  attributable event in the same transaction. The explicit `not_started` payment
  and fulfilment states avoid a false claim that acceptance has collected money
  or instructed a kitchen.
- The contract correctly treats quote expiry as a first-placement deadline only,
  and keeps receipt access independent of current publication state. Terms and
  original placement output are not rewritten by the new operation record.

## Findings

| Priority | Evidence | Finding and required correction |
| --- | --- | --- |
| Medium | `contracts/openapi/v1/paths/merchant-food-orders.yaml`, `merchant-food-order.yaml`, `food-order-decisions.yaml`, `food-order-cancellation.yaml`, and `admin-food-order.yaml` | The new operations mostly declare only `default`, despite `contracts/README.md` requiring expected errors to be listed per operation. Clients need the actual 400/401/403/404/409/503 boundary, especially the two stable 409 order codes. Add the shared Problem response to each applicable explicit status; retain `default` only for unforeseen failures. |
| Medium | `internal/persistence/order_operations.go:89-117`; `domains/food/order/v1/schema.yaml:48-54` | A single-order read returns every event, while the contract promises `history.maxItems: 4`. This is true in this batch only because there can be at most placed plus one decision/cancellation event. Before any later payment/work transitions are added, either bound and cursor the returned history or remove the maximum and document the retention/read limit. Do not let a future valid order become an invalid response. |
| Medium, next-slice gate | `internal/persistence/order_operations.go:89-117` | Acceptance is deliberately just a merchant decision here. That is acceptable for this bounded batch, but the implementation does not recheck current Food availability or allocate any configured capacity when the merchant accepts. Before automatic acceptance, payment-gated acceptance, or kitchen work, make the recorded policy explicit and recheck the relevant feasibility at the serialized acceptance boundary. Do not retrofit this by reinterpreting the quote/order's retained policy or price. |

## Verification targets

The existing integration scenarios are well chosen: distinct audience/scope
denial, guest ownership, stale revisions, replay after later progress, restart,
rollback, withdrawal-independent reads, provider read-only access, and competing
actions. The implementation run should additionally ensure each documented
explicit error response is exercised once after the OpenAPI response correction.

## Bounded verdict

No authority or lifecycle **blocker** is visible in the reviewed design. The
slice preserves the intended distinction between placement, merchant decision,
payment, and physical fulfilment. Resolve the explicit-response contract issue
before accepting this batch; retain the availability/configuration observation as
a gate for the next automatic-acceptance or preparation slice, rather than
expanding this batch into payments or kitchen work.

## Verification update

**Snapshot:** follow-up static review on 21 October 2026 after the reported
response, conflict, and history changes. No shared Docker, database, or browser
task was run by this reviewer.

- **Resolved:** `OrderRevisionConflict` now carries the authoritative current
  operational revision, and `write` emits `order_revision_conflict` with
  `current_order_revision`. This gives a client enough information to read or
  deliberately retry; it does not blur a stale revision with an invalid state
  transition.
- **Resolved for the present state set:** individual reads take `FOR SHARE OF s`
  while mutations take `FOR UPDATE OF s`, retaining the lock while the event
  history is read. A response therefore does not combine one operation revision
  with a later history. The `maxItems: 4` limit remains valid only while the
  current maximum is placement plus a single decision or cancellation. The
  documented pagination/limit work is a required gate before payment or kitchen
  events add further history.
- **Remaining correction (Medium):** expected Problem responses are now
  explicit, but the three read operations overstate their contract by declaring
  `409`, `413`, and `415`: `merchant-food-orders.yaml`,
  `merchant-food-order.yaml`, and `admin-food-order.yaml`. These GET operations
  have no state transition or request media type, so they should list the errors
  clients can normally receive from their declared inputs and authorization
  boundary (`400`, `401`, `403`, `404`, and `503`), while retaining `default` for
  unforeseen failures. The two POST mutation operations appropriately retain
  the full `400`/`401`/`403`/`404`/`409`/`413`/`415`/`503` set. This is contract
  precision, not a request for another product capability.

### Conditional disposition

The updated revision conflict, read consistency, replay, authority, and retained
terms semantics are acceptable for Batch 2. Accept the protocol portion after
narrowing those GET response declarations and confirming the already-running
checks pass. No payment, kitchen, automatic acceptance, or provider-transfer
scope should be pulled into this batch.

## Final static disposition

**Snapshot:** follow-up static review on 21 October 2026 after the final
source-contract and inbox-cursor corrections. Runtime checks remain owned by the
implementation run.

- **Resolved:** the three GET operations now declare only the applicable
  `400`/`401`/`403`/`404`/`503` Problem responses. The state-changing POST
  operations continue to declare their possible conflict and request-body
  failures. This gives generated clients a truthful distinction between a read
  boundary and a command boundary.
- **Resolved:** the merchant inbox is explicitly newest-first by immutable
  `created_at` with order ID as a deterministic tie-breaker. Its encoded cursor
  carries that same position, is bounded and validated, and is documented as an
  opaque continuation value rather than an authorization credential. This is
  compatible with live-page semantics; newly placed orders are deliberately
  found by refreshing the first page rather than expected to appear mid-walk.

**Protocol disposition: accepted, contingent only on the already-running
generation, integration, and browser checks passing.** The current slice has no
remaining protocol blocker. The stated next-slice gates still apply: bound or
page individual-order history before additional events, and establish serialized
availability/policy handling before automatic acceptance, payment gating, or
preparation work.
