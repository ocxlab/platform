# Protocol review: domain admission and generated-contract policy

**Review date:** 2026-10-22
**Scope:** Static review of the uncommitted domain-admission and generation-policy changes. This review did not execute shared Docker, database, browser, or full task checks.

## Reviewed boundary

The change introduces an explicit OpenAPI `x-commerce-boundary` classification:

- `shared` routes remain available independently of an installed commerce model.
- `food` routes require the Food domain.
- `model` routes require at least one enabled offering-model implementation and therefore cannot reach storage when the installation has no enabled model.

The handler builds Food validators and projections only when Food is enabled, mounts the Food MCP surface only in that state, and rejects model-bound operations before authentication or persistence when no model is installed. Startup validates that every declared path has a recognized boundary.

## Findings and disposition

| Finding | Disposition |
| --- | --- |
| Generic offering, publication, public listing, handle, media, and stream paths could previously remain reachable when Food was disabled because path-level Food classification did not cover them. | Resolved. These are now `model` routes, and the `none` installation test exercises them as unavailable. |
| A disabled domain could still have an active validator or public projection through a registry constructed independently of enabled domains. | Resolved for the current Food-only installation. The registry is built from enabled implementations only; the `none` tests cover validation and projection rejection. |
| Removing generated artifacts from Git could leave a clean checkout unable to build or make stale generated contracts appear valid. | Addressed by ignored generated paths, generation in the local and Docker workflows, and a tracked-artifact guard. The parent reports that `generated:check` is now part of `task check` and passed in the current run; this review did not independently execute it. |

## Required boundary for a second domain

This is sound for a single enabled Food model. It is not, by itself, a multi-domain isolation mechanism. Before Shopping or Lodging is installed, every model-bound read and write must resolve the resource's exact registered `(model.id, model.version)` and enforce that model's enabled domain. Mixed public listings, handle lookups, media, streams, and publication projections must filter or reject models outside the installation's enabled set. The current documentation states this prerequisite; it must remain a release gate for the second domain.

## Verdict

**Accepted for the present Food-only capability boundary, conditional on the parent-run generation and full-check results.** The implementation makes no Beckn, ONIX, UCP, or multi-domain compatibility claim. The stated second-domain gate is essential rather than optional future hardening.
