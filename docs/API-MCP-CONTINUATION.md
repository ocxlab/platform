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
   SSE-driven invalidation. This is planned work, not an installed dependency.
6. Build provider administration on that foundation, then continue the planned
   Beckn/ONIX work. Notify the founder before admin design so their screenshots
   can inform it. Keep existing catalog/network behavior working throughout.

These are logical surfaces, not new mandatory services. Food tools stay in Food;
account capabilities stay in Core. Reassess endpoint grouping when another commerce
domain gains tools; do not add empty Shopping/Lodging MCPs now.

## Verification and boundaries

Keep OpenAPI modular, generated artifacts ignored, stable safe error codes, tenant
scope, idempotency, and bounded OTel labels. Verify authorization at invocation as
well as discovery. Use focused race/integration tests, full `task check`, and
targeted mutation tests for behavioral changes. Record what actually ran, remaining
gaps, and review findings after each coherent batch. Model-free SDK/Inspector tests
do not establish conversational task quality.

No admin redesign, new frontend library, external model call, or production OAuth
rollout is implied by starting the first catalog-fidelity increment.
