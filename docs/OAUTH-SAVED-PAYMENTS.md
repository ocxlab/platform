# OAuth and saved-payment implementation boundary

24 October 2026. This is the next security-sensitive product slice, not a claim
that production login or processor charging is implemented. Complete the checkout
regression and reviewed frontend-state pass first; administration follows later.

## One identity, distinct authority

Use a maintained authorization server, with authorization-code flow and PKCE,
instead of building an OAuth server inside OCP. Map a verified issuer/subject pair
to a Core account; never identify or link an account from an agent-supplied ID or
an unverified email. REST and MCP can share this identity and application policy,
while retaining different resource audiences and least-privilege grants.

For MCP, expose protected-resource metadata and meaningful bearer challenges.
Validate signature, issuer, intended resource, expiry, and delegated scopes on
each request, then enforce our own account/merchant/store permissions. An ID token
is not an API access token. Do not forward the incoming bearer to arbitrary tools
or payment processors. Existing local opaque grants remain development fixtures;
they do not become OAuth merely by changing a header or OpenAPI security scheme.

This follows the MCP specification's resource-server model and target-resource
requirements, not an assumption that an identity provider implements every
required feature. See [MCP authorization, 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization),
Roles, Authorization Server Discovery, and Token Handling.

Supabase is a candidate because the founder previously mentioned it. Its current
documentation describes an OAuth 2.1 server, consent UI, and MCP authentication.
Before adopting it, test the actual issued audience/resource and scope behavior
against our separate Go REST/MCP resource servers; an example protecting Supabase
data does not prove our resource isolation. No Supabase project has been created.
Sources examined: [OAuth server overview](https://supabase.com/docs/guides/auth/oauth-server)
and [MCP authentication](https://supabase.com/docs/guides/auth/oauth-server/mcp-authentication).

The web app should keep its session and tokens server-side/HttpOnly. Login,
logout, or account change creates a new principal epoch, cancels private queries,
and clears their caches before displaying the next account's data. Existing guest
orders remain scoped to their original guest authority; adopting them into an
account requires explicit authenticated proof, not name matching.

## Saving an instrument is not consenting to a purchase

The payment service owns processor-customer and payment-method references, bound
to the authenticated account and processor/merchant context. Store only opaque
references and safe display metadata, never PAN/CVC or processor client secrets.
Collection and authentication belong in the processor's hosted/embedded surface.
Only a verified server-side processor result may mark a setup complete; browser
completion alone is not proof. Handle duplicate, reordered, and forged callbacks.

Stripe's SetupIntent is a candidate mechanism for saving without charging. Its
documentation distinguishes setup from a later PaymentIntent and notes possible
customer authentication. See [SetupIntent API](https://docs.stripe.com/api/setup_intents).
Processor choice and a test-mode account are still required. Do not infer that a
saved method can be reused across unrelated merchant payment accounts.

Each payment must separately identify an owned order, immutable agreed amount and
currency, selected usable instrument, and explicit consent. OAuth scope is not
unlimited spending authority. Durable idempotent payment work must handle extra
customer authentication, timeout/unknown outcome, retry, cancellation, and refunds;
do not report success merely because a processor accepted a request. Saved-method
management needs its own modular OpenAPI resources and account-scoped MCP intentions.

## Acceptance gates

Exercise a real local/test authorization-code flow, code/state/PKCE rejection,
signature/key rotation, issuer/audience/scope/expiry rejection, logout/cache
isolation, and merchant/customer separation. Then verify saved-method setup,
cross-account denial, duplicate callbacks, removal, an explicitly consented
test-mode charge, authentication-required and declined outcomes, durable retries,
and order/payment recovery. Preserve guest checkout and native forms throughout.

These are planned checks. None has been executed for OAuth or saved instruments
in the product. A provider preference and test configuration have been requested
from the founder; do not substitute simulated OAuth or development payments for
those acceptance results.
