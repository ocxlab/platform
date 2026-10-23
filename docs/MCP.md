# Local merchant MCP

Owners can use an MCP client to find their stores, search menu summaries,
inspect a dish's sizes and selected extras, change availability or prices in a private
draft, and review exact changes between retained revisions. The adapter shares the Offering
write operation with REST; it does not forward arbitrary API calls. Publication
has a separate exact-revision preview and explicit confirmation operation.
The [tool contract](../contracts/mcp/v1/merchant/README.md)
records inputs, permissions, retries, compact outputs, and errors.

Extras inspection preserves default choices and size-dependent surcharge overrides.
Focused price edits can change a base surcharge or a named size override without
replacing the rest of the menu. Meal offers have a compact overview/detail read
and a focused create/update tool, including eligibility, add-on choices, and
upgrade surcharges. Focused dish/category setup and reviewed publication are
also implemented. Merchant order inbox/detail, decisions, and pickup progression
use the same authorized Food application operations as REST. The separate
[Customer Food MCP](CUSTOMER-MCP.md) now supports browse/configure/price/commit
and own-order progress. Core's [account interfaces](ACCOUNT-MANAGEMENT.md) support
personal profile reads and preference edits through separate REST/MCP audiences;
they are not production identity or saved-payment management.

## Try it locally

Run `task dev`, then `task dev:stores` to find an existing store handle. Obtain a
one-hour development MCP credential with `task dev:mcp-credential STORE=store-2`
(substitute your handle). This prints the token explicitly for local use, not to
ordinary API logs. It covers that store's owning merchant, including their other
stores, using the current scope-filtered tools. It is not a UI login or REST credential.
New credentials include order read/manage in addition to catalog scopes, exposing
eighteen tools. Existing restricted grants are not upgraded automatically; issue
a fresh development credential when testing the expanded merchant workflow.

Connect your client to `http://127.0.0.1:18180/mcp/merchant`, select Streamable
HTTP, and supply `Authorization: Bearer <token>`. The Compose stack enables
`OCP_LOCAL_MERCHANT_MCP=true`; other runtime configurations leave it disabled.
The endpoint accepts localhost Host values and either no Origin or its own
exact Origin. It is not enabled for arbitrary web origins or remote deployment.

For repeatable schema inspection, set `OCP_MCP_TOKEN` in your current shell and
run `task mcp:inspect`. This uses native Node/pnpm and pinned MCP Inspector
2.12.0; the provider and database stay in Docker. Task prints the variable name,
not its expanded token. Do not paste credentials into committed configuration.
The first Inspector run downloads its client dependencies. No model key is needed.

## Transport and authentication

Go MCP SDK **v1.8.0**, commit
`3f3b699b2b67e1ed033a63d6651671dab53c2d32`, is pinned in `go.mod`/`go.sum`.
The adapter uses stateless Streamable HTTP with JSON responses, 262144-byte
request limits, and a ten-second request deadline. GET/DELETE are not persistent
session channels. This does not change the separate order-status SSE API.

The released SDK enforces modern 2026-07-28 request metadata and mirrored
headers. Scripted HTTP tests include the protocol version and client capabilities
in `_meta`, plus `Mcp-Protocol-Version`, `Mcp-Method`, and `Mcp-Name` for tool calls.
Prefer an SDK client rather than hand-assembling these headers. Inspector 2.12.0
was exercised in both `modern` and `legacy` modes; the SDK client was exercised
over real HTTP, not just an in-memory transport.

REST and MCP now use the same merchant grant ownership, scopes, expiry,
revocation, and application authorization policy. They have **different bearer
resources**, `merchant-rest` and `merchant-mcp`; neither credential works at the
other boundary. Existing REST grants retain their resource during migration.
Provider-admin and guest grants remain separate. The database stores token hashes,
not plaintext tokens; authenticated requests use the restricted application role.

This is a local bootstrap mechanism, **not OAuth**. Production delegated access
still requires a chosen authorization server and the MCP resource-server rules
for discovery, resource/audience validation, consent, and scoped grants. No fake
OAuth metadata, home-grown authorization server, external identity provider,
or universal API/MCP token has been added.

## Visibility and remaining work

The OTel observer records the fixed MCP HTTP route plus tool calls, latency,
in-flight counts, and bounded outcomes. A tool rejection inside HTTP 200 is
recorded as a tool rejection. Known tool names/codes are allowlisted; unknown
names become `_OTHER`. Arguments, catalog bodies, credentials, merchant/store
identifiers, and raw error text are not telemetry labels or failure logs.
Export remains off by default. These are per-attempt signals, not unique business
mutation totals, durable audit records, model token usage, or retained admin stats.

The [intent and information-disclosure design](MCP-TOOL-DESIGN.md) describes
the implemented read path and the broader catalog still to build.
The next slices are the separately designed
customer MCP, and the **MCPs** provider-admin area for management and bounded
operational summaries. Merchant/customer account tools require their corresponding
account capabilities first. The embedded assistant and live model tests also
remain later work. No AI task-quality, public OAuth, browser Inspector UI, load,
or production-security test is claimed here.

### Tool-design checkpoint before expansion

The initial three tools were a transport/authorization test slice, not the final
merchant catalog or an argument for minimizing tool count. Before expanding it,
review merchant and customer intentions separately across commerce and account
work. Tool boundaries should make those tasks straightforward and safe, not
match REST paths or compress unrelated actions into a generic dispatcher.

Here progressive disclosure means **information**: store overview, matching dish
summaries, one dish's sizes/prices and extras-group summaries, then selected
groups' options if needed. This is not a requirement for progressive loading of
tool definitions. Nine tools now cover that read path, draft availability,
batched dish prices, meal-offer inspection/configuration, and field-change review.
Menu creation/import, other corrections, approval/publication tools, and account
workflows remain unfinished.

Measure the complete workflow cost—tool schemas, arguments, result size, and
extra round trips—rather than treating three tools or short responses as proof
of token efficiency. A larger set of well-scoped tools can be more efficient.
Include the 48-item menu, variants and free/paid extras, multilingual data,
ambiguity, conflicts, and denied permissions in that review. Batch related details
when that avoids unnecessary round trips; do not optimize response size alone.
Keep discovery deterministic and permission-filtered. Client-side schema loading
is a separate future concern, not the meaning of information disclosure here.

## Release evidence

See [MCP media and interaction findings](MCP-MEDIA-AND-INTERACTIONS.md) before
adding basket state, rich menu images, elicitation, Apps, caching, or Tasks.
Those mechanisms have distinct scopes and client-support requirements.

Checked on 22 October 2026, Europe/Berlin (UTC+02:00):

- [SDK v1.8.0 transport options](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/streamable.go), `StreamableHTTPOptions`; [protocol metadata validation](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/shared.go), `validateRequestMeta`.
- [MCP HTTP authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization), resource-server/authorization-server roles, resource metadata, and token audience validation.
- [Inspector CLI](https://github.com/modelcontextprotocol/inspector/blob/main/clients/cli/README.md), Remote Servers and `--protocol-era`; executable release 2.12.0 was checked with `--help` and run locally. Its npm tarball integrity is `sha512-ppfN5+HnVWo6dY7jz5RMFUFTfEYPnFUjN3XN2w6PI6nmB/AFFXPGPfLBRvCeSx+qIH6kpi7Nsz5HYRp49loHpA==`.
- [SDK release license](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/LICENSE): the file describes an Apache-2.0/MIT transition with documentation treated separately. Do not reduce that statement to a blanket license for every material. Inspector is a development tool, not vendored application code.
