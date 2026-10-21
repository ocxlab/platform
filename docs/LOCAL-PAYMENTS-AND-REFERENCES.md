# Local checkout and readable references

The local development payment service lets you exercise an order without a
processor account. Its financial operation belongs to the Payment module; it
runs inside the existing provider process and persists in PostgreSQL. It is a
simulator, not a Stripe emulator or evidence of real settlement.

## Try an order

Run `task web:dev` for local Vite with the Docker API/database, or
`task web:container` for the containerized UI. Existing development data survives
migrations. Use the homepage's published-store list to enter as a guest.

1. Select a meal, request its price, and place a pickup order.
2. Open that order in the merchant workspace and accept it.
3. In the customer receipt, click **Refresh status**, then **Pay order**. This
   redirects to a separate development checkout page, not inline receipt controls.
4. Choose success, decline, or abandonment, then return to the receipt. Returning
   without choosing an outcome leaves payment pending; **Continue to checkout**
   resumes it. Decline and abandonment allow another
   attempt, up to ten checkout starts per order.
5. Refresh the merchant view to see the persisted financial result. Cancelling an
   unprepared simulated paid order records a simulated refund atomically.

Acceptance does not collect money or start preparation. After simulated payment,
the merchant can **Start preparation**, **Mark ready for pickup**, then **Record
customer pickup**. Each step persists and appears on the customer receipt after
refresh. Automatic cancellation is unavailable once preparation begins; customer
support and legal rights are not removed by this UI limitation.

Customer and merchant refresh links reload current server state,
including when clicked repeatedly on the same URL; no polling or push is implied.

The local tasks enable `OCP_DEVELOPMENT_PAYMENTS=true` for API and web processes.
Restart an already running Vite process once after this setting is introduced.
The backend defaults to disabled outside this local configuration. Never enable
the simulator as a real-money payment method in production.

## What is durable

Checkout progress, immutable amount/currency, order events, and successful
idempotency responses commit together under the order's operational lock.
Reopening a receipt or restarting an API handler retains that state. A stale
revision conflicts rather than overwriting newer progress. Retrying a successful
operation returns its original response; refresh reads the current order.

The guest must still possess the scoped credential. Neither a UUID nor a readable
reference authorizes access. Separate guests, merchant credentials, and anonymous
callers cannot mutate another guest's payment operation. Future real-processor
adapters must implement their own financial outcomes and compensation; they do
not inherit the development adapter's automatic simulated refunds.

## References people can read

Orders display **O-000001**; merchant workspaces show store handles such as
**store-1**. Public store cards show business names, not bare internal handles.
Each reference uses a persistent
provider-wide database sequence, not a daily counter. It is therefore unique
within this provider across dates without requiring people to remember a date.
Six order digits are a minimum, not a limit. Numbers may have gaps and are never
deliberately recycled. Existing records receive numbers during migration.

These are presentation references, not new global identities or credentials.
Store names can repeat and change; handles remain stable. The published-store
directory accepts an exact handle search. API paths and authenticated ownership
continue to use UUIDs. Editable usernames, custom storefront domains, and
cross-provider naming are separate future work.

Retained placement and idempotency response bodies are not rewritten to add a
reference: current reads supply it. This preserves the original retry contract.

## Verification boundary

Database integration tests exercise lifecycle/retry/restart behavior, simulated
refunds, concurrent completion versus cancellation, authorization, immutable terms,
readable references, and runtime-role ownership checks. Browser regression tests
exercise both views. These checks do not establish real processor compatibility,
bank settlement, or physical kitchen readiness. See [mutation testing](MUTATION-TESTING.md)
for the separately bounded fault-injection scope.

## Real provider checkout

The product currently implements **only the development adapter**. Stripe and
Mollie sandbox experiments in `ocp-lab` do not make them product adapters.

The common customer journey is **Pay → checkout → return to order**. The
[Stripe Checkout](https://docs.stripe.com/payments/checkout) and
[Mollie hosted checkout](https://docs.mollie.com/docs/accepting-payments)
documentation describe hosted redirects; Stripe also offers embedded Checkout,
and Mollie offers provider-specific components. Those embedded integrations are
provider-specific; sharing the OCP checkout handoff does not make their widgets
interchangeable.
The simulator follows the handoff pattern without requesting card details or
pretending to be either provider. A real hosted page keeps its provider's UI.

Actual adapters still require merchant/payee binding, persisted provider session
references, provider-side idempotency, authenticated outcome retrieval/webhooks,
and reconciliation of interrupted attempts. Browser redirects never establish a
financial outcome. Real cancellation/refunds must not inherit simulated atomic
refund behavior. Guest cookies also need a deliberate cross-site return policy
before external checkout is wired; the current local simulator is first-party.
