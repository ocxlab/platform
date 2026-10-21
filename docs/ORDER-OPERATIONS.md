# Restaurant decisions and order progress

This increment gives restaurants an inbox for purchases already placed by guests.
Staff inspect the agreed selection and accept or decline it. Customers see the
result on their private receipt and can cancel while an order is open and
unprepared, subject to the implemented payment method's cancellation rules.
The development method supports simulated refunds. The same operations are available through the authorized JSON
APIs; the web application does not have a privileged alternative path.

## Ownership and state

The Food domain owns acceptance and resolution. The existing placement body and
quoted terms remain immutable; a separate operational row holds the current
revision, acceptance, resolution, and fulfilment progress. Merchant decisions and guest cancellations
lock that row, check its revision, and commit their event and replay result in
one PostgreSQL transaction. Read-side shared locks keep progress and its event
history consistent even when an action arrives during a read.

| Action | Required state | Result |
| --- | --- | --- |
| Merchant accepts | Open, pending acceptance, payment/preparation not started | Accepted, still open |
| Merchant declines | Open, pending acceptance, payment/preparation not started | Rejected and closed |
| Placing guest cancels | Open, preparation not started; unpaid or development payment | Cancelled and closed; simulated payment is refunded when applicable |
| Merchant starts preparation | Open, accepted, paid pickup order, preparation not started | Preparing |
| Merchant reports ready | Open, accepted, paid pickup order, preparing | Ready for pickup |
| Merchant records pickup | Open, accepted, paid pickup order, ready | Collected; fulfilment ends |

Each action requires the current operational revision and a scoped retry key.
Repeated successful attempts return their original result, not a reconstructed
current order. Read the order to obtain its latest state. A different request
cannot reuse the key, and a late action cannot reopen a closed order.
Audit records retain actor and request identifiers; the returned history contains
only event, sequence, and time. It does not expose guest tokens or actor IDs.

## Walkthrough

Start with an existing published store and place a pickup order through its public
page. Open **Order inbox** in the merchant workspace, open the order, and accept
or decline it. Refresh the customer's receipt to see the response. An accepted,
unpaid order can still be cancelled by its placing guest; refresh the merchant
page to see the closure. Provider administration can inspect a known order with
its own `order:inspect` credential, but cannot accept it or invent payment evidence.

The inbox is newest-first and paginated. Refresh shows new arrivals; this
increment does not provide automatic notifications. Existing order access survives
publication withdrawal and does not depend on the original temporary quote.

`task dev` stops the old local API before running migrations and starting its
replacement. This prevents legacy placement code from inserting an order after
the progress backfill without creating its operational row. It does not reset the
database. The brief development interruption is not a zero-downtime production
rollout strategy; production migration compatibility remains a release concern.

## Deliberate next-slice boundaries

Acceptance is not payment evidence or a kitchen instruction. Merchant-reported
preparation/readiness/collection now complements the development checkout; it
does not move real money or independently prove physical work. See the
[local payment and pickup walkthrough](LOCAL-PAYMENTS-AND-REFERENCES.md).
Real payment adapters must retain unfinished work and reconcile uncertain
external effects rather than inherit simulated atomic refunds. Before adding
more events, reconsider the 32-entry history bound.

Automatic acceptance, store opening/ordering controls, preparation estimates,
timeouts, merchant delivery, customer notifications, and support beyond the
seven-day guest grant remain product work. Automatic acceptance must check its
configured feasibility and manual item availability; it must not silently reprice
agreed terms. Tracked inventory is not required for the initial manual-availability
mode. Neither founder inspection nor a later payment callback may reopen a closed
order or substitute for the merchant's fulfilment decision.

## Verification

Real PostgreSQL coverage includes audience/action/tenant boundaries, guest
ownership, stale revisions, immutable terms, matching and conflicting retries,
concurrent acceptance/preparation versus cancellation, atomic replay/event rollback, restart,
newest-first pagination, and receipt access after withdrawal. The browser journey
joins merchant acceptance, checkout, guest cancellation, merchant rejection,
preparation through pickup, and read-only administration. See [status](STATUS.md) for the checks actually completed and the
two automated technical review reports under [reviews](reviews/).
