# OCP Platform implementation workspace

This private monorepo contains the maintained implementation, not the research
corpus or an already standardized protocol. Preserve user edits. Never push,
publish, register services, or use real payment credentials without authorization.

## Development

- Use the root `Taskfile.yml` as the command interface. No CI configuration yet.
- Organize runtimes under `services/`: `provider/` for the owned Go implementation,
  `onix/` for retained network-runtime packaging/configuration. Put UIs under
  `apps/`, contracts under `contracts/`, and multi-component composition under
  `deploy/`. Logical module boundaries need not be separate runtimes.
- Keep provider `go.mod`, `cmd/` executable entry points, and `internal/` private
  packages inside `services/provider/`. Do not spread language-specific source
  directories across the root. Actual automation scripts belong under `scripts/`
  if needed; `cmd/` is not a build-script directory.
- Run builds, formatting, tests, and development services in Docker. Do not require
  host Go, PostgreSQL, Python, or a cloud account. Task and Docker are host tools.
  Exception: the user requests local Node/pnpm for Svelte/Vite hot reload through
  `task web:dev`; backend/database and reproducible checks remain in Docker.
- Keep commands portable across Windows, Linux, and macOS. Put shell-specific
  implementation details in containers, not host-only scripts.
- Pin dependencies and retained artifacts. Do not copy private research snapshots,
  sandbox secrets, experimental code, or third-party schemas without checking
  provenance and applicable material-specific rights.
- Do not select a production cloud or add mandatory brokers, Kubernetes, separate
  replicas, or cross-cloud infrastructure to a development scaffold.

## Design boundaries

The product serves merchants and customers through an OCP commerce service,
identity service, discovery service, and separately scoped payment service.
Logical boundaries do not require separate deployments. Food is an independent
domain; its lifecycle must not become the universal commerce lifecycle.

Both merchant and customer commerce APIs act on the same purchase records.
Catalog administration belongs to Offering, not the purchase engine. Payment owns
financial operations; discovery holds public projections, not private orders.
Preserve tenant scope, agreed terms, authorized replay, and durable work alongside
decisions. Do not fake readiness or payment success.

All supported business REST APIs will have version-controlled OpenAPI contracts
and appropriate generated artifacts. UI, external API, and future agent adapters
must use the same authorized application operations. Structural code generation
does not implement behavioral rules or authorization.

Build web applications with SvelteKit and TypeScript. Add provider administration
incrementally alongside the capabilities being managed, not as a separate late
dashboard project. Use a separately authorized, OpenAPI-defined provider
administration API in the existing backend; never give the UI direct database
access. Record consequential administrative actions, preserve tenant scope, and
test denied access as well as supported management actions. Do not introduce
unrestricted impersonation, secret exposure, or mandatory extra deployments.

Edit modular source under `contracts/openapi/v1/`, not the ignored bundle or
generated Go. Keep reusable resource definitions, parameters, headers, and errors
in their owning files and expose stable root component names. After changes, run
`task contracts:generate`, `task web:generate`, and `task check`; commit source and
generated Go/TypeScript artifacts together. All operations declare `x-access`;
protected operations also declare a recognized `x-required-scope` and matching
security scheme. Keep merchant and provider-admin credential audiences separate.
Read `contracts/README.md` before extending the contract or offering-model registry.

Prefer a compact modular implementation with a growth path. Initial storage uses
the selected managed provider's recovery facilities; extra protected copies and
isolated backup storage are optional later tiers. Federation is a committed later
stage, not an excuse to omit network integration or prebuild live merchant moves.

## Verification and writing

Run `task check` for changes and `task smoke` when changing runtime/container wiring.
Report only checks actually run and their limits. Update `docs/STATUS.md` after
coherent batches. Do not mistake a running health endpoint for commerce readiness.
Explain purpose and ownership before details; keep names consistent across prose,
tables, and code. Use one semantic category per table column.

Use conventional commits with type, scope, and an imperative subject. Inspect staged
changes first. Use the configured Git identity without changing it or adding
assistant/co-author/sign-off attributions. Do not push.
