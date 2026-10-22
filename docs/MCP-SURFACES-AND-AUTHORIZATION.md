# Four intent surfaces with scoped authorization

22 October 2026, Europe/Berlin (UTC+02:00). Product design recommendation based
on primary documentation, not an implemented MCP or identity integration.

## Organize tools around the people using them

MCP is an agent-facing interface, not an automatic REST export. Share business
logic and security rules beneath the interfaces, but design tools from concrete
tasks. Maintain four logical surfaces even if only two endpoints expose them:

| Surface | Intent examples | Authority boundary |
| --- | --- | --- |
| Merchant commerce | Correct menu prices, review draft changes, publish the reviewed menu, handle an order. | Specific merchant, permitted stores, and permitted operations. |
| Merchant account | Update business contact details, manage store membership, manage delegated access. | Account-management rights distinct from menu or order rights. |
| Customer commerce | Find suitable food, configure an order, place it, check progress, request cancellation. | Public browsing or authority over the customer's own purchases; separate payment consent. |
| Customer account | Manage profile details, saved addresses, preferences, and delegated access. | The customer's account, not arbitrary customer or merchant records. |

These examples describe future coverage, not existing account APIs. Sensitive
account-security actions need individually justified tools and stronger checks;
do not expose every account operation to a model simply because a UI supports it.
Keep provider administration outside both ordinary merchant/customer surfaces.

Recommend `/mcp/merchant` and `/mcp/customer` as two Streamable HTTP resource
endpoints within the existing provider process. One endpoint per audience keeps
tool catalogs focused and credentials separated without four deployments.
Account and commerce tools remain separate modules and scope groups under each.
Four endpoints are possible if a later trust/client boundary warrants them; one
endpoint is also technically possible. Endpoint count is not authorization.

Use small, explicit tool schemas rather than a universal `call_api` or generic
`account_action` tool. Read tools offer bounded search, field selection where
useful, pagination, and concise summaries. Mutation results report changed items,
revision, warnings, and next action instead of returning the whole catalog.
Evaluate definitions, inputs, outputs, and total round trips against complete
tasks. Token economy must not hide material prices, errors, or consent terms.

## What MCP authorization contributes

MCP's HTTP authorization specification uses OAuth-based delegated access. The
authorization server issues a token for the target MCP resource; the MCP server
validates it on each request. Scopes express permitted operations, while our
application still checks ownership, store limits, and any purchase constraints.
The specification covers resource discovery, target-resource identification,
token validation, and insufficient-scope responses [S1].

A person can have both merchant and customer roles, but granting one must not
silently grant the other. Likewise, menu editing need not authorize account
security changes. A token is not an unrestricted account login. Payment limits
and saved-payment use require application/payment authorization in addition to
permission to invoke a purchase tool.

Return only the tools relevant to the endpoint and granted authority. MCP permits
`tools/list` to vary with request authorization [S2]. This reduces irrelevant
definitions, but hiding tools is not access control: enforce the same policy on
every invocation. Keep deterministic ordering and prevent cross-user tool/result
cache reuse. Never derive the acting account from an unverified model argument.

Public customer discovery can be unauthenticated. Private account actions and
registered purchases need appropriate authorization; a guest purchase can retain
its own narrow capability without creating a customer account. Exact guest MCP
client authorization and the public OAuth rollout still need implementation
design and compatibility tests. Local scoped development credentials are not a
claim of full OAuth conformance.

## Where x401 could fit

The examined x401 specification is **draft 0.2.0**, pinned to commit
`7e864af97cd8202a5380386f52939d759541977d` [S3]. It carries credential-proof
requirements over HTTP and offers optional token exchange. Payment is separate;
agent binding is optional. Its future-work section still discusses delegation.
Therefore it does not supply our complete merchant/customer permission model.

Potential later use: accept a trusted organizational credential as evidence in
an authorization decision. Account mapping, permission limits, and consent remain
our responsibility. Possessing an attribute proof is not automatically permission
to edit a merchant's menu or spend a customer's money. Keep proof verification
as an optional authorization input, not a requirement for ordinary ordering.

Do not assume any MCP client automatically handles x401 proof headers or its
retry flow. Such composition needs a client/authorization integration test.
The repository README and changelog use `presentation_requirements`, whereas the
pinned normative draft uses `credential_requirements`; use the pinned draft for
this assessment, not README payload examples [S3, S4]. No x401 exchange was run.

## Next design artifact

Before implementation, define the first merchant use cases and a compact tool
catalog: intent, arguments, compact result, required authority, mutation/approval
behavior, and retry semantics. Start with commerce setup; add account intentions
as their underlying product capabilities become available. Design customer tools
from customer journeys separately, rather than reuse the merchant tool catalog.

## Evidence

All sources accessed 22 October 2026, Europe/Berlin. Specification statements
above are separated from our endpoint/tool recommendations. No runtime tests.

- S1: [MCP Authorization, 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization), Roles, Scope Selection Strategy, Resource Parameter Implementation, Token Handling, and Scope Challenge Handling.
- S2: [MCP Tools, 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/server/tools), Capabilities, `tools/list`, and Calling Tools. Its authorization-dependent listing rule supports filtering; it does not replace application policy.
- S3: [x401 pinned specification](https://github.com/proof/x401/blob/7e864af97cd8202a5380386f52939d759541977d/spec.md), header status/version, Abstract, Design Goals, Agent Binding Options, Access Token Acquisition, and Open Questions & Future Additions. Repository head resolved through GitHub's public commits API before fetching the pinned file.
- S4: [Repository README](https://github.com/proof/x401#readme), How it works; [changelog](https://github.com/proof/x401/blob/main/CHANGELOG.md), 0.2.0 Added. Both were examined as supporting material, not preferred over the normative draft.
