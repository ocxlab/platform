# OCP Platform

A private monorepo for the managed OCP service: commerce, catalogs, accounts,
network discovery, identity, payment integration, and eventually the applications
merchants and customers use. These responsibilities can run compactly at first
without losing their boundaries as the service grows.

The current slice connects private menu editing, exact-revision publication,
guest browsing, and server-priced selection quotes. Merchants and provider admins
use separately authorized JSON APIs backed by PostgreSQL; SvelteKit supplies their
interfaces and the public storefront. Modular OpenAPI contracts generate Go and
TypeScript artifacts. Guest pickup-order placement and private receipts are
available. Acceptance, kitchen operations, payment, and native ONIX integration
remain next slices; this is not yet a complete ordering product.

## Start locally

Install Docker with Compose and [Task v3](https://taskfile.dev/docs/installation).
No host Go installation, cloud account, or payment key is required.
For Svelte hot reload, install Node.js 22.12+ (or Node.js 24) and pnpm 11.20.0.
Use `task web:container` instead if you prefer no host Node installation.
Docker must be running with Linux containers.

```text
task doctor
task check
task smoke
task web:dev
```

`task smoke` starts the stack and leaves it running. Open
<http://localhost:18180/health/live>. The response establishes process liveness,
not order readiness. There is intentionally no successful readiness claim.
PostgreSQL has no published host port; access it through Compose.

Open <http://localhost:18181> for the web application. Run `task dev:credential`
and enter the printed merchant ID and temporary token at `/merchant`. Create a
store, save a menu, and publish its saved revision; the workspace links to its
public storefront. `task dev:admin-credential` supplies a separate temporary
credential for `/admin`, where merchant/store IDs locate publication for inspection,
suspension, or restoration. These are local development bootstraps, not production
signup or login. Tokens are printed once; do not commit them.

```text
task logs
task stop
```

`task stop` removes the stack's containers/network but preserves its database and
compiler-cache volumes. No destructive reset task is provided. Local database
credentials in Compose are development-only and are not a production account model.

`task web:dev` runs the Svelte/Vite development server locally with hot reload;
leave its terminal running and stop it with Ctrl+C. It uses the Docker API/database
and keeps the same web URL. `task web:container` runs the built web image instead;
stop the local Vite process before switching to it. For backend edits, run
`task dev` to rebuild/restart the API. `task stop` stops Docker, not local Vite.
The HTTP port can be changed in an optional `.env`, using `.env.example` as a guide.
The repository works with the defaults without creating that file. Keep real
credentials out of source control; none are needed for the current checks.

## Commands

| Task | Purpose |
| --- | --- |
| `task` | List commands |
| `task doctor` | Check host tools and Compose syntax |
| `task fmt` | Format Go sources inside Docker |
| `task contracts:generate` | Validate/bundle modular OpenAPI and regenerate committed Go |
| `task contracts:check` | Check generated-code drift without rewriting committed Go |
| `task web:generate` | Regenerate TypeScript API types from the shared OpenAPI bundle |
| `task check` | Check contracts, formatting, vet, race/SQL tests, web types/components, and runtime builds |
| `task test:browser` | Run Chromium merchant/guest/admin journeys with disposable PostgreSQL |
| `task web:dev` | Start Docker API/database and local Svelte hot reload on localhost:18181 |
| `task web:container` | Start the built web app in Docker without hot reload |
| `task test` | Run unit tests |
| `task test:race` | Run unit tests with the race detector |
| `task test:integration` | Exercise authorized HTTP/SQL behavior in disposable isolated databases |
| `task db:prepare` | Explicitly provision development role and apply embedded migrations |
| `task dev:credential` | Print a new development merchant and temporary bearer credential once |
| `task dev:admin-credential` | Print a separate temporary provider-admin credential once |
| `task dev` | Build/start API and PostgreSQL, waiting for container health |
| `task db:check` | Run a harmless SQL connectivity query |
| `task smoke` | Start the stack and check actual HTTP and SQL connectivity |
| `task stop` | Stop the stack without deleting persisted data |

There is no CI configuration. The same Task commands can be called by CI later.
First builds need registry access for the pinned images; later dependency updates
may require authorized module downloads. Go module dependencies are pinned in
`services/provider/go.mod` and `go.sum`. Go and PostgreSQL versions/digests are recorded in the provider Dockerfile
and local Compose file;
they are tested development pins, not a promise to use those versions forever.

## Layout and next increment

- `services/provider/`: owned Go API/worker runtime; its own module and tests.
- `services/provider/cmd/`: API, local checks, contract tooling, migration, and development provisioning entry points.
- `services/provider/internal/`: private Go implementation packages.
- `services/onix/`: retained network runtime boundary and integration plan.
- `apps/web/`: SvelteKit merchant workspace, public storefront, and provider administration.
- `deploy/local/`: local composition of independently runnable components.
- `contracts/`: modular versioned OpenAPI, shared resources, Food definitions, and generation instructions.
- `docs/IMPLEMENTATION-KICKOFF.md`: ownership, repository growth, and acceptance milestones.
- `docs/STATUS.md`: executed checks and unfinished work.

The module path `ocp.local/platform` is a local import namespace. It does not
register a domain, select a Git hosting organization, or define a merchant identity.
Inside `services/provider/`, Go's `cmd/` convention groups executable entry points, not build
scripts; `internal/` marks packages whose imports are restricted by Go. They are
not repository-wide categories. Build tasks remain in the root Taskfile; actual
future automation scripts can go under `scripts/` if needed.

The provider runtime and retained ONIX runtime are separate components. The first
does not claim to replace the second; no native network integration runs yet.
The provider can initially contain several logical service modules without
requiring a server per module. See `services/README.md` for the structure.

Start a foundation API session with `task dev`, then `task dev:credential`. Use
the returned merchant identifier and bearer token with the operations described in
[contracts/README.md](contracts/README.md). Keep the token private; it expires in
24 hours. Provisioning is a local administrative command, not public signup.
The API uses the restricted `ocp_app` database role; migration/provisioning use a
different administrative connection. Do not reuse development credentials in production.

Read [the publication slice report](docs/PUBLICATION-SLICE.md) for behavior,
executed checks, and remaining boundaries. The next increment connects quotes to
a restaurant purchase journey, not unrelated placeholder endpoints. Production
login, exact public protocol cohorts, and production hosting remain later decisions.
The selected foundation cohort and deferred boundaries are in
[docs/FOUNDATION-DECISIONS.md](docs/FOUNDATION-DECISIONS.md).

Research stays in the sibling `ocp-lab` repository. Its implementation-design
baseline is commit `878e242` and its final report is under
`implementation-design/reviews/completed-design-synthesis.md`. Local development
does not read that repository or its environment files. No research snapshots,
third-party runtime code, or credentials have been imported here, and no project
license or publication permission is implied by creating this repository.
