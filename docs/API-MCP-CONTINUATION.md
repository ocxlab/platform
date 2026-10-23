# API/MCP continuation

23 October 2026. Resume product work after the completed modular-monolith batches.
The goal is to complete useful merchant and customer intentions through the same
authorized application operations as REST, without turning MCP into an endpoint
dispatcher. Existing implementation and gaps are recorded in [tool design](MCP-TOOL-DESIGN.md).

## Implementation order

1. Reconcile the existing merchant tools with the current Food menu model. Preserve
   size-dependent extra prices, default selections, and meal-offer semantics in
   focused reads and edits. Test immutable revisions, atomic changes, and replay.
2. Complete merchant Food intentions: bounded menu setup/corrections, explicit
   reviewed-revision publication, and order handling. Repair shared REST/application
   gaps rather than implement different agent-only business behavior.
3. Add customer Food intentions: published-store/menu discovery, focused item
   configuration, basket changes, quote/commit, and own-order progress. Preserve
   material prices and explicit purchase consent. Account-backed saved payments
   require their own authorization implementation; a tool grant is not payment consent.
4. Add Core account-management operations and audience-scoped tools as capabilities
   become available. Merchant/customer account authority remains separate from
   commerce authority. Public OAuth is an explicit implementation/security gate;
   current local development credentials are not OAuth.
5. Implement the committed XState/TanStack frontend foundation before the admin
   redesign. Assess state transitions and server-data ownership separately, choose
   the relevant packages, and migrate a tested vertical slice before expanding.
   Do not duplicate server data in two stores or assume either library enforces
   backend authorization. Cover failure/retry, stale responses, navigation, and
   SSE-driven invalidation. The first receipt slice and its pinned dependency
   choices are described in [frontend state ownership](FRONTEND-STATE-RESEARCH.md).
6. Build provider administration on that foundation, then continue the planned
   Beckn/ONIX work. Notify the founder before admin design so their screenshots
   can inform it. Keep existing catalog/network behavior working throughout.

These are logical surfaces, not new mandatory services. Food tools stay in Food;
account capabilities stay in Core. Reassess endpoint grouping when another commerce
domain gains tools; do not add empty Shopping/Lodging MCPs now.

The first increment now preserves size-dependent extras and defaults. Meal-offer
support adds bounded summaries/details and focused create/update commands using
the same Food model validation and Offering write as REST. The three merchant
batches now add focused dish/category setup, exact reviewed publication, and
order inbox/detail, decisions, and pickup progression. Food owns shared order
application operations for REST and MCP. Offer deletion, ordering-policy setup,
production account identity and saved-payment tools remain further work; these
batches do not complete the API/MCP phase. Customer Food tools and bounded Core
profile/preferences tools are now implemented; see their linked contracts and
development status for verified behavior. Full-merchant local MCP credentials now include
order read/manage; existing restricted credentials are not silently upgraded.

## Verification and boundaries

Keep OpenAPI modular, generated artifacts ignored, stable safe error codes, tenant
scope, idempotency, and bounded OTel labels. Verify authorization at invocation as
well as discovery. Use focused race/integration tests, full `task check`, and
targeted mutation tests for behavioral changes. Record what actually ran, remaining
gaps, and review findings after each coherent batch. Model-free SDK/Inspector tests
do not establish conversational task quality.

Frontend libraries are now introduced through the separate, researched receipt
slice, not as part of catalog-fidelity changes. Admin redesign still waits for
the founder's screenshots. External model calls and production OAuth remain
separate setup and authorization gates.

## Retained MCP evaluation before handoff

Keep three testing layers distinct: deterministic application/HTTP/SDK regression
tests, protocol/client interoperability checks, and model-driven task evaluations.
The first layer already runs in the test suite. Inspector has been exercised in
earlier batches; that does not establish model task quality or full conformance.

Before declaring the API/MCP phase complete, retain at least one executable,
model-driven end-to-end scenario and its deterministic companion. First scenario:
an authorized merchant asks to change only the large pizza's extra-cheese surcharge
in a fictional test store, inspect the saved result, and summarize the changes
without publishing. The agent must discover the right store/item/group/size rather
than receive a prescribed tool-call sequence.

Grade the actual retained state and tool trace: correct target and requested price,
unchanged base/other-size prices and unrelated dishes, one draft change, and no
publication or unauthorized action. Grade the final explanation as a separate
quality check, not the sole evidence of success. Add ambiguous-item and stale-revision
variants as the tool set expands. A customer configure/quote/commit scenario follows
once those tools exist, using simulated payment and explicit purchase consent.

Use isolated resettable fixtures, bounded calls/tokens/time/spend, and synthetic data.
Record the model/version, parameters, prompt, tool schemas/build, fixture version,
calls, sanitized results, latency, tokens, and per-assertion outcomes. Repeat live
runs to expose variability; one pass is a reference run, not a reliability estimate.
Do not retain bearer grants or credentials in traces. Local transport/business
assertions remain runnable without a model key; live evaluations are opt-in and
must be reported as not executed until actually run with an approved provider/key.

Tooling candidates checked on 23 October 2026:

- [MCP Inspector smoke-test guidance](https://github.com/modelcontextprotocol/inspector/blob/main/docs/cli-smoke-testing.md)
  distinguishes shell smoke checks from conformance; retain the existing pinned client.
- [Official MCP conformance framework](https://github.com/modelcontextprotocol/conformance),
  Server Testing and Conformance Requirements, is a separate protocol-testing candidate.
  Select applicable scenarios for our pinned protocol and advertised capabilities;
  do not claim OAuth conformance for the local bootstrap endpoint.
- [lastmile-ai/mcp-eval](https://github.com/lastmile-ai/mcp-eval), Architecture and
  examples, is an evaluation-runner candidate with trace-based assertions.
  [Promptfoo's MCP integration](https://www.promptfoo.dev/docs/integrations/mcp/)
  is an alternative. Neither is selected or installed yet: first verify our
  stateless Streamable HTTP protocol era, authentication headers, local agent loop,
  trace/state assertions, licensing, and pinned dependency compatibility. Keep the
  scenario/assertions separable from the runner rather than depend on an LLM judge.
