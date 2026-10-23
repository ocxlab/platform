# Notification-driven worker implementation review

23 October 2026. Two AI reviewer roles inspected commit `9ae979b`: principal
engineering and protocol/commerce architecture. Both read the implementation,
SQL, tests, ownership manifest, and evidence notes. They did not execute tests;
the implementation agent ran the verification commands recorded in
[development status](../STATUS.md). These are separate AI role inspections, not
independent human assessment or interoperability evidence.

## Verdict and scope

Both reviewers accepted the bounded `food/catalog.publish` implementation. Neither
found a blocking correctness or security defect. They confirmed that SQL effects
remain authoritative, domain-only notifications and Go channels are disposable
hints, and retries/expired leases retain durable recovery. External delivery stays
outside transactions; claim exclusivity does not establish exactly-once delivery.

The inspected seams preserve the inexpensive combined runtime and allow later
transport/process changes without pretending that Redis solves atomic publication.
Existing SSE remains separate. Payment, kitchen dispatch, public network trust,
and independent protocol conformance are outside this acceptance.

## Findings and disposition

| Finding | Disposition |
| --- | --- |
| Engineering: reconnect delay resets after any successful LISTEN, so repeatedly dropped connections keep retrying every second. | Fixed: repeated short-lived attempts now use 1, 2, 4, 8, 16, then 30 seconds. A successful attempt lasting at least 30 seconds resets the delay. Cancellation still interrupts waiting. Unit tests cover growth, cap, reset, and cancellation. Engineering reread the change and accepted the fix. |
| Engineering follow-up: make the 30-second stability policy explicit. | Documented in the worker implementation note. |
| Protocol: explicitly assign the trigger function owner rather than relying on the role applying migration 19. | Deferred nonblocking hardening. The function is trigger-only, not SECURITY DEFINER, has a fixed search path, and has PUBLIC execution revoked. No historical migration was rewritten. A future corrective migration can pin its owner and assert it alongside the existing ACL checks. |

The engineering follow-up also inspected the stronger attempt tests: 30-second
lease, ten-second handler deadline, three-second detached cleanup, retained effect
identity, retry/final failure, storage errors, rejected completion, and metric
outcomes. No remaining consequential flaw was identified.

## Handoff limits

Adapters must honor cancellation; Go cannot safely force-stop arbitrary handler
code. Repeated external attempts require receiver idempotency or reconciliation.
Multiple workers can wake for the same hint, and notification/aggregate-query cost
must be profiled before increasing replicas or adopting a broker. The local
idle/burst observations establish scheduling behavior, not cloud costs or capacity.

Mutation scope, surviving mutations, and exclusions are recorded separately in
[mutation testing](../MUTATION-TESTING.md). Neither review approval nor mutation
efficacy substitutes for database isolation/recovery tests.
