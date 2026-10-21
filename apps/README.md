# Web applications

This is the application boundary of the monorepo, separate from backend runtimes
under `services/`. Use SvelteKit with TypeScript for the merchant application,
customer storefront, and provider administration application. Add directories
when implementing working journeys, not empty framework scaffolds. Package
tooling and exact dependency versions will be pinned with the first UI slice;
Docker supplies the tooling and root Task commands remain the entry point.

Merchant setup/catalog administration and order handling, plus the customer
storefront, must use the supported application APIs rather than private database
access or a parallel UI-only purchase lifecycle. Add actual application directories
when building those journeys; do not fill them with untested framework scaffolds.

## Provider administration

Add management capabilities in the slice that introduces the underlying function.
For each slice, identify the minimum administration needed, or record why none is
needed yet. Candidate operations include merchant/store lookup, publication or
access suspension, credential revocation, failed-work inspection, and supported
recovery actions as their owning features are implemented. This is not permission
to ship all of those operations before their business behavior exists.

Use a dedicated provider administration API with modular OpenAPI contracts and a
generated TypeScript client. It may run in the existing Go backend. Provider
administration authorization is separate from merchant permissions; consequential
actions need an attributable audit record and appropriate retry/concurrency
safeguards. Read-only views should expose only the data needed for the task, never
payment secrets. No direct database access or unrestricted impersonation.

Share useful accessible UI components across applications without coupling their
permissions or creating another commerce lifecycle. Keep commerce rules behind
the supported APIs so future native apps and agent clients use the same operations.

## Verification

Use Vitest and Svelte Testing Library for focused UI behavior, and Playwright for
browser journeys against the actual API/database. Include authorization rejection
and administrative audit behavior in backend/integration checks, plus keyboard
and accessibility checks for the UI. Run browser tooling in Docker through Task.
These are agreed implementation requirements, not tests already executed.
