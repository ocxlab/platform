# Account profiles across REST and MCP

Account management lets merchants and customers inspect their personal profile
and change its display name and preferred application language. Core owns those
records and the shared application operations. A merchant's personal profile is
not their trading identity or store name; editing it does not change a menu,
order, commerce permission, or payment method.

This is a local development foundation. Accounts are explicitly provisioned
fixtures, not verified production identities. Signup, identity-provider login,
OAuth delegation, account recovery/deletion, addresses, and saved payment methods
remain separate capabilities. Existing browser merchant login and guest checkout
are unchanged; a profile does not silently adopt guest purchases.

## Interfaces and authority

| Surface | Profile REST resource | Local MCP endpoint |
| --- | --- | --- |
| Merchant account | `/api/v1/merchant-account/profile` | `/mcp/account/merchant` |
| Customer account | `/api/v1/customer-account/profile` | `/mcp/account/customer` |

GET returns the authenticated profile. PUT replaces its editable fields using
`expected_revision` and `Idempotency-Key`. The modular OpenAPI sources under
`contracts/openapi/v1/schemas/accounts/` and their resource path files define
the wire contract; generated artifacts stay ignored.

Each MCP offers `inspect_my_profile` followed by `update_my_preferences`.
The latter takes the inspected revision, personal display name, `de`, `en`, or
`ar` locale, and a retry key. These are the two implemented intentions, not
generic access to every account record or a claim that two tools complete account
management. Account tools are separate from Food and future Shopping/Lodging tools.

Four resource audiences distinguish merchant/customer and REST/MCP. Tokens are
hashed at rest, expire, and can be revoked. A distinct `AccountPrincipal` type
cannot be passed as a commerce principal. `account:read` permits inspection;
updates require both read and write scopes. A merchant commerce, provider-admin,
or guest credential is not an account credential.

Every operation rechecks its grant inside an account-scoped transaction.
PostgreSQL RLS also limits profile, grant, retry, and audit access to that account.
Updates serialize on the profile, check successful retries before revisions,
and commit the profile, minimal actor/request/revision audit, and replay together.
A matching retry returns the original body; changed content with the same key
conflicts. Fresh reads return the current profile. The audit does not duplicate
previous personal names or preferences. Retention remains a release decision.

## Try a fixture locally

After `task dev`, create a customer account and one-hour MCP credential:

```sh
task dev:account-credential KIND=customer NAME="Example customer"
```

Keep the returned account ID. Issue a separate REST credential for the same
profile with `ACCOUNT=<id> TRANSPORT=rest`. To create a merchant profile, use
`KIND=merchant NAME="Example owner" MERCHANT=<existing-merchant-uuid>`.
Omitting ACCOUNT creates a new fixture; it does not identify someone by their name.
The command checks that its database is `ocp_dev` and prints the new secret only
as explicit bootstrap output. Never put it in source files or ordinary logs.

The Compose API enables `OCP_LOCAL_ACCOUNT_MCP=true`. Like Food MCP, these
endpoints accept localhost Host values, enforce same-origin requests, bound body
size and execution time, and use stateless Streamable HTTP with no-store replies.
The shared technical transport does not own account or commerce authorization.
Public deployment still requires the planned OAuth resource-server boundary.

OTel records fixed tool/surface names and bounded outcomes, not profile values,
credentials, or account IDs. See [development status](STATUS.md) for executed
verification and its limits.
