# Merchant tools and conversational onboarding

Research checkpoint: 22 October 2026, Europe/Berlin (UTC+02:00).
This is a bounded product-design investigation, not an extension of the original
twelve-candidate standards review. No MCP server, chat integration, or external
model call was executed in this batch.

## Recommendation and decision criteria

Build merchant tools over **Streamable HTTP MCP**, then test embedded onboarding
with **Vercel AI SDK in SvelteKit**. Feature coverage takes precedence over the
existing stack: retain SvelteKit because the examined documentation supports our
workflow, not because changing frameworks is forbidden. There is no documented
blocker requiring a framework change at this checkpoint. Installed-package and
end-to-end compatibility still need tests.

The useful workflow is not merely exchanging chat messages. Owners need to select
a store, describe or import an offering, correct the proposed draft, and publish
only after reviewing it. Tools must preserve the same authorization, revisions,
and retry behavior as ordinary API clients. A fluent conversation cannot repair
a missing or excessively coarse business API.

| Required capability | Evidence and implication | Implementation check |
| --- | --- | --- |
| Network-accessible merchant tools | Streamable HTTP defines one MCP endpoint; official Go SDK documents an HTTP handler [M1, M3]. | Serve `/mcp` alongside the provider; no mandatory new runtime. |
| Streamed chat in our UI | AI SDK documents SvelteKit endpoints, the Svelte `Chat` class, and streaming [A1]. | Verify native Vite and built Node/Docker behavior. |
| Multi-step tool use | Core tool calling supports bounded loops and cancellation [A3]. | Exercise listing, reading, editing, and rereading the substantial demo menu. |
| Chat uses the actual MCP interface | AI SDK has an HTTP MCP client [A2]. | Connect the server-side chat route to our allowlisted MCP endpoint. |
| Review before publication | SDK tool approvals and UI approval states are documented [A3, A4]; Svelte inherits the shared chat implementation [A5]. | Verify approval rendering and enforce publication authorization server-side. |
| Ordinary external clients | Inspector documents graphical and scripted HTTP clients [M4]. | Prove the tool journey without any model or AI key first. |
| Provider choice | AI SDK allows replacing the tutorial's gateway provider [A1]; its OpenAI adapter calls OpenAI directly [A6]. | Start with direct OpenAI access; do not require Vercel hosting or Gateway. |
| Multilingual merchant UX | Our existing UI uses Fluent for German, English, and Arabic. | Test conversation quality, RTL presentation, and corrections separately; SDK support is not evidence of language quality. |

## Transport and authorization

Use Streamable HTTP, not the superseded standalone HTTP+SSE transport. For MCP
revision `2026-07-28`, POST requests receive JSON or request-scoped SSE responses;
protocol sessions and the old standalone GET stream are removed [M1]. This is
separate from our existing order-status SSE interface. A synchronous tool result
does not require opening a permanent event stream.

The Go SDK's current documentation covers both protocol eras and requires its
stateless handler setting for `2026-07-28` [M3]. AI SDK and Inspector also describe
modern/legacy negotiation [A2, M4]. These are documentation observations, not a
tested release combination. Pin released SDK, Inspector, and AI SDK packages as
one compatible set before implementation; do not copy older initialization or
session examples into a modern-only server. Narrow supported protocol revisions
to those actually exercised.

Prefer a thin Go adapter in the existing provider runtime. It should invoke the
same authorized application operations as REST, with explicit input/output
validation and a small tool allowlist. This preserves the API boundary without
making a loopback HTTP request or forwarding client tokens unnecessarily.
Shared operation semantics, not a particular internal hop, are the requirement.
If shared application operations need extraction from HTTP handlers, make that
bounded refactor rather than duplicate business rules.

For the first localhost-only development slice, issue an explicitly scoped
development credential for the MCP resource. Authenticate each request, retain
tenant/scope/revocation checks, and reject REST-only or administration credentials.
Do not describe this as implementing public MCP OAuth. Public access needs the
MCP authorization discovery and resource-specific token rules [M2]. An adapter
must not accept an unrelated API token and blindly pass it downstream [M5].

Bind published development ports to loopback, validate supplied Origin headers,
and configure explicit allowed origins where browser access is needed [M1].
Container-internal listening and host-published exposure are different settings.
The embedded chat route chooses the MCP destination from trusted configuration,
not user input. Preserve request limits, timeouts, safe errors, request IDs, and
business idempotency across retries. Closing a stream does not prove that an
already committed change was cancelled.

## Chat should assist the merchant, not own the catalog

Use `ai`, `@ai-sdk/svelte`, `@ai-sdk/mcp`, and the direct `@ai-sdk/openai` provider
as candidate packages. Current fetched AI SDK documentation identifies itself as
v7; exact install versions and peer dependencies remain to be selected. Its
repository LICENSE identifies Apache-2.0 [A7]; check the selected release and
dependency notices when installing rather than infer all dependency rights.

The browser displays streamed text, tool activity, and proposed changes. The
SvelteKit server performs model calls and authenticated MCP calls; credentials
never enter the browser bundle or model context. Keep the existing forms and
save durable drafts through the business API, not through conversation memory.
Imported menu descriptions and model output are untrusted input. Validate tool
arguments and minimize merchant data sent to the model.

Publication remains an explicit merchant decision. SDK approval controls apply
to locally executed tools, not automatically to provider-executed tools [A3].
Start with the server-side SDK MCP client rather than delegating tool execution
to a model provider. Also, client-supplied chat history is not proof of approval:
the SDK documents protection against fabricated approvals [A4]. Bind the approved
action to the authenticated merchant, store, and reviewed revision on our server.
If that is not ready, keep publication in the ordinary review screen rather than
pretend a prompt instruction enforces it.

Set turn/tool-step, duration, input-size, and output-token limits. Test bounded
recovery, cancellation, denied tools, model errors, and lost streams without
erasing the saved draft. Start with fictional menu data. Photo interpretation,
voice conversations, and autonomous customer ordering are separate feature tests,
not implied outcomes of this research. Change the relevant library or framework
if an essential feature fails its acceptance test; do not drop the feature just
to preserve a stack choice.

## Keys now and secrets management later

An application API key is separate from ChatGPT Pro subscription usage [O1].
No key is needed for Inspector or model-free MCP tests. Before live chat tests,
use a dedicated API project/key with separately configured API billing. Load
`OPENAI_API_KEY` only in the server runtime from an ignored local environment file;
never use a `PUBLIC_`/`VITE_` variable, commit it, or paste it into chat. The existing
repository ignores `.env` and `.env.*` except `.env.example`. Provide the exact
file path and Task command when that slice is wired, not before a loader exists.

Record **Infisical EU Cloud** as the preferred later central secrets manager.
Its own published asset list identifies `eu.infisical.com` as the EU region [I1].
Use it when multiple environments, collaborators, credential rotation, or CI
make copied environment files cumbersome. Machine identities can provide scoped
workload access [I2]. Keep the application consuming runtime configuration so
Task/Docker/CI can later inject values without changing business code. Do not
make every order depend on a new remote vault lookup. Application service keys,
user grants, and merchant payment credentials have different owners and lifecycles;
centralizing infrastructure secrets does not resolve all three.

No Infisical account, integration, or migration is needed now. EU secrets hosting
does not establish EU-only model processing or complete regulatory compliance;
confirm service terms and model data handling separately before real merchant use.

## Implementation sequence and stop condition

1. Pin a compatible release set; implement scoped Streamable HTTP merchant tools
   and Task-based Inspector/scripted checks. Include errors, wrong tenant,
   insufficient scope, revoked credentials, stale revisions, and replay.
2. Exercise the 48-product setup/edit/review journey. Fix coarse API operations
   or unsafe error/retry classifications discovered through tools.
3. Add a feature-gated embedded assistant using the same HTTP MCP tools, first
   with deterministic model mocks, then a separately authorized live model test.
4. Verify progressive review/publishing, failure recovery, language presentation,
   and cost limits in both native development and built-app tests.

Stop the initial slice once merchants can complete and correct a draft through
tools and chat without bypassing authorization or accidental publication. Do not
wait for ONIX, public OAuth rollout, voice, or future customer purchasing tools.

## Evidence register

All entries were accessed on 22 October 2026, Europe/Berlin, during this research
session. IDs are local to this product note. Live documentation and `main` code
are mutable observations, not pinned implementation artifacts or empirical proof
of reliability. Only short paraphrases are retained; no downloaded code is reused.

| ID | Issuer and source | Exact locator |
| --- | --- | --- |
| M1 | MCP: [Streamable HTTP, 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http) | Introduction; Security & Endpoint; Backward Compatibility. |
| M2 | MCP: [Authorization, 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization) | Access Token Usage; resource-server validation. |
| M3 | MCP Go SDK: [Protocol documentation](https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/protocol.md) | Lifecycle; Discovery; Stateless Mode; Resumability and Redelivery. |
| M4 | MCP: [Inspector](https://modelcontextprotocol.io/docs/2026-07-28/tools/inspector) | Quickstart; HTTP CLI example; secret storage warning. |
| M5 | MCP: [Security Best Practices](https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/security_best_practices) | Token Passthrough; SSRF. |
| A1 | Vercel: [Svelte getting started](https://ai-sdk.dev/docs/getting-started/svelte) | Install Dependencies; Create an API route; Wire up the UI; Multi-Step Tool Calls; Svelte/React differences. |
| A2 | Vercel: [MCP client](https://ai-sdk.dev/docs/ai-sdk-core/mcp-tools) | Initializing an MCP Client; HTTP Transport; protocol-era compatibility. |
| A3 | Vercel: [Tool Calling](https://ai-sdk.dev/docs/ai-sdk-core/tools-and-tool-calling) | Tool Execution Approval; Multi-Step Calls; Abort Signals. |
| A4 | Vercel: [Chatbot Tool Usage](https://ai-sdk.dev/docs/ai-sdk-ui/chatbot-tool-usage) | Server Setup; Client-Side Approval UI; Securing Approvals for Sensitive Tools. |
| A5 | Vercel: [Svelte Chat source](https://raw.githubusercontent.com/vercel/ai/main/packages/svelte/src/chat.svelte.ts) and [shared Chat source](https://raw.githubusercontent.com/vercel/ai/main/packages/ai/src/ui/chat.ts) | `Chat extends AbstractChat`; `addToolApprovalResponse`. Source-level support, not a compiled test. |
| A6 | Vercel: [OpenAI adapter source](https://raw.githubusercontent.com/vercel/ai/main/packages/openai/src/openai-provider.ts) | `createOpenAI`; default API base URL and `OPENAI_API_KEY` loading. |
| A7 | Vercel: [AI SDK LICENSE](https://raw.githubusercontent.com/vercel/ai/main/LICENSE) | Apache License, Version 2.0 notice. |
| O1 | OpenAI: [Pricing](https://learn.chatgpt.com/docs/pricing) | Pricing introduction: API token prices separate from subscription usage; Token rates: API-key usage follows API pricing. |
| I1 | Infisical: [Published asset list](https://infisical.com/vulnerability-disclosure) | Scope / In scope: EU Cloud hostname. Consulted only as hostname/region evidence, not as instructions to perform security testing. |
| I2 | Infisical: [Machine identities](https://infisical.com/docs/documentation/platform/identities/machine-identities) | Organization and project identities; setup and authentication methods. |

Retrieval limitations: guessed Infisical `/docs/cloud/overview` and
`/docs/documentation/platform/cloud` pages were unavailable; its documentation
index led to the current deployment guidance instead. The AI SDK OpenAI-provider
HTML exceeded the retrieval limit and its Markdown response was unsupported by
the web reader; the official adapter source supplied direct-provider evidence.
The guessed AI SDK `/reference/ai-sdk-ui/chat` route was unavailable; the Svelte
guide, shared source, and `use-chat` reference supplied the relevant API evidence.
No claim of exhaustive ecosystem search or production maturity benchmarking is made.
