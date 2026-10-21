# Checkout and pickup protocol review

Reviewed 22 October 2026, Europe/Berlin (UTC+02:00), against the working tree
after `dbf340d`. Automated static review by the existing protocol reviewer;
no shared Docker tests were run by this reviewer.

No remaining semantic blocker was identified. Checkout navigation carries no
payment evidence; only the guest-authorized backend mutation changes the local
simulation. Disabled flags, tenant scope, revision checks, and replay semantics
are retained. A real provider requires its own persisted session and outcome
verification rather than inheriting this simulator's authority model.

Merchant-reported pickup progress remains distinct from payment and acceptance.
The transition guard is serialized with cancellation. A collected order cannot
restart preparation, and collection is not claimed as independent physical proof.
`collected` is a fulfilment terminal state; `resolution` remains a separate axis.

The reviewer requested a same-revision cancellation/preparation race. The added
integration test requires exactly one success and one conflict, ending either
cancelled/refunded/unprepared or open/paid/preparing, never cancelled/preparing.
Additional replay assertions cover readiness and collection. The expanded
32-entry history bound accommodates ten checkout attempts and pickup progress;
further events require reconsidering that bound.
