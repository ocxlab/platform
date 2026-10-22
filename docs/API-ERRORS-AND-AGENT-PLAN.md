# API errors and an early agent-facing slice

22 October 2026, Europe/Berlin (UTC+02:00). Product audit and plan, not part of
the twelve-candidate review. MCP, Inspector, and chat are not implemented yet.

## Existing error boundary

Clients receive HTTP error statuses with `application/problem+json`, stable
module-owned `code` values, safe details, and matching request IDs. Generated Go
and TypeScript share modular OpenAPI sources. The server validates requests;
application operations enforce ownership, scope, business rules, concurrency,
and authorized replay. Schema validation cannot replace those checks.

Catalog checks detect status/code mistakes; integration tests validate response
schemas. Fluent covers English/German/Arabic and unknown-code fallback. Internal
exceptions are not customer messages. 401 carries a bearer challenge; capacity
responses include Retry-After. The audit corrected the generic `about:blank`
title to use the HTTP status phrase, leaving the specific category in `code`,
following [RFC 9457 §4.2.1](https://www.rfc-editor.org/rfc/rfc9457.html#section-4.2.1).
Its §§3.1.4, 3.2, and 5 also inform safe details and extensions.

Remaining improvements: structural validation gives generic `invalid_request`,
not field-level JSON pointers. Operations list expected HTTP statuses, but not
a complete inventory of application codes. Add safe field diagnostics and code
examples during the agent slice. Never expose validator dumps, submitted secrets,
or raw provider failures. Keep codes open for additive evolution.
Unexpected application/storage errors currently share a conservative 503 fallback.
Separate internal faults from identifiable transient dependency failures before
relying on automated retry policies; a safe message does not establish retryability.

## Next: merchant setup through MCP and Inspector

Do this before chat, without waiting for ONIX or final UI polish. The first
journey lists/selects or creates a merchant's store, reads its menu, saves a
draft, reviews it, and explicitly publishes it. Include ordering controls.
Use the 48-product fictional restaurant to exercise choices, sizes, extras,
and unavailable products.
In particular, check whether whole-menu reads and replacements are too coarse
for small edits. That can justify bounded item/category operations in the API
instead of asking a model to reconstruct every unchanged product.

Design a small intent-based tool catalog from merchant use cases, sharing the
authorized business operations beneath REST rather than exporting its endpoint
catalog. Use agent-friendly arguments, bounded reads, compact mutation results,
and runtime input/output checks. No database shortcut,
arbitrary destination URL, provider-admin authority, or automatic exposure of
every endpoint. Preserve merchant scope, revisions, and idempotency keys.
Conflicts require rereading, not silent overwrites. Structured tool errors retain
safe application codes and correlation IDs.
Start with a local Streamable HTTP merchant endpoint, using an explicitly scoped
development credential for that resource. This supersedes the earlier stdio-first
proposal. Public OAuth authorization remains a separate deployment increment.
Use the same authorized application operations as REST; do not blindly pass
REST credentials through an MCP proxy. The [transport and chat investigation](MCP-AND-CHAT-RESEARCH.md)
records protocol-version compatibility, authentication, and acceptance checks.
The [four-surface clarification](MCP-SURFACES-AND-AUTHORIZATION.md) recommends
merchant and customer endpoints, each with separately scoped account and commerce
tools. These are logical surfaces, not four mandatory services. Customer tools
follow customer intentions and do not reuse the merchant tool catalog.

Pin a compatible SDK/protocol/Inspector cohort and add Task commands to run the
adapter, launch Inspector locally, and run scripted checks, using Docker where
practical. No MCP dependency or task was installed in this batch.
References consulted:

- [MCP tools](https://modelcontextprotocol.io/specification/2026-07-28/server/tools):
  schemas, tool results/errors, and user interaction.
- [Inspector](https://modelcontextprotocol.io/docs/2026-07-28/tools/inspector):
  interactive/CLI testing, Quickstart, and Secrets storage. Keep Inspector local
  and test grants out of Git and logs.

Acceptance checks cover setup end to end, invalid input, stale revisions,
successful retry after a lost response, wrong tenants, insufficient scope,
and revoked credentials. Draft saving must not publish; publication remains an
explicit reviewable action. Record missing operations, awkward payloads, and
ambiguous recovery as API findings. Repair the API rather than hide gaps in
agent-only code. Inspector checks establish tool/API behavior, not conversational
usability.

## Then add embedded onboarding chat

A development-only assistant uses the same authorized operations to help create
a draft. Merchants review names, prices, sizes, and extras before publishing.
Show proposed changes and tool activity, preserve work on failure, and retain
ordinary editing. Imported menu text is untrusted data, not instructions.
Never put credentials in prompts. Choose the model/provider and data-handling
settings before sending real merchant content externally.

Evaluate Vercel AI SDK with its Svelte bindings and HTTP MCP client first. Feature
coverage takes precedence over preserving a framework choice. Direct OpenAI API
access does not require Vercel Gateway; API usage is separate from ChatGPT Pro.
Keep keys in ignored, server-only runtime configuration initially. Prefer
Infisical EU when growing secrets/environment/CI needs justify centralization.
No AI SDK dependency, API key, or Infisical integration was added by the research.

Customer ordering tools follow the cart/checkout work. Multilingual conversational
testing remains separate; neither delays the initial model-free setup journey.

Instrument the MCP and shared REST application paths with OTel alongside tool
implementation. Add a provider-admin MCPs tab for endpoint management, calls,
latency, and safe failure details; no Grafana stack is required initially. See the bounded
[observability plan](OBSERVABILITY-AND-MCP-OPERATIONS.md), including privacy,
storage, failure classification, and verification requirements.
