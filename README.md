# OCP Platform

A private monorepo for the managed OCP service: commerce, catalogs, accounts,
network discovery, identity, payment integration, and eventually the applications
merchants and customers use. These responsibilities can run compactly at first
without losing their boundaries as the service grows.

This first increment is the **local development foundation**, not a restaurant
product. It provides an API process with a liveness endpoint, PostgreSQL, and
containerized build/test commands. It does not yet connect the API to PostgreSQL,
implement business operations, run ONIX, or integrate payment processors.

## Start locally

Install Docker with Compose and [Task v3](https://taskfile.dev/docs/installation).
No host Go installation, npm install, cloud account, or payment key is required.
Docker must be running with Linux containers.

```text
task doctor
task check
task smoke
```

`task smoke` starts the stack and leaves it running. Open
<http://localhost:18180/health/live>. The response establishes process liveness,
not order readiness. There is intentionally no successful readiness or business
endpoint yet. PostgreSQL has no published host port; access it through Compose.

```text
task logs
task stop
```

`task stop` removes the stack's containers/network but preserves its database and
compiler-cache volumes. No destructive reset task is provided. Local database
credentials in Compose are development-only and are not a production account model.

Edit code, then run `task dev` to rebuild/restart it. Hot reload is not configured.
The HTTP port can be changed in an optional `.env`, using `.env.example` as a guide.
The repository works with the defaults without creating that file. Keep real
credentials out of source control; none are needed for the current checks.

## Commands

| Task | Purpose |
| --- | --- |
| `task` | List commands |
| `task doctor` | Check host tools and Compose syntax |
| `task fmt` | Format Go sources inside Docker |
| `task check` | Check formatting, run vet and race tests, and build the API image |
| `task test` | Run unit tests |
| `task test:race` | Run unit tests with the race detector |
| `task dev` | Build/start API and PostgreSQL, waiting for container health |
| `task db:check` | Run a harmless SQL connectivity query |
| `task smoke` | Start the stack and check actual HTTP and SQL connectivity |
| `task stop` | Stop the stack without deleting persisted data |

There is no CI configuration. The same Task commands can be called by CI later.
First builds need registry access for the pinned images; later dependency updates
may require authorized module downloads. The current Go code uses only the standard
library. Go and PostgreSQL versions/digests are recorded in the provider Dockerfile
and local Compose file;
they are tested development pins, not a promise to use those versions forever.

## Layout and next increment

- `services/provider/`: owned Go API/worker runtime; its own module and tests.
- `services/provider/cmd/`: executable entry points (`api`, `check-local` today).
- `services/provider/internal/`: private Go implementation packages.
- `services/onix/`: retained network runtime boundary and integration plan.
- `apps/`: merchant/customer application boundary; frontend tooling not selected.
- `deploy/local/`: local composition of independently runnable components.
- `contracts/`: rules for introducing versioned API contracts.
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

The first business increment will implement one complete restaurant journey,
contract-first, rather than independent placeholder APIs. Frontend framework,
OpenAPI generators, exact public protocol versions, authentication formats, and
production hosting remain bounded decisions before their respective implementation.

Research stays in the sibling `ocp-lab` repository. Its implementation-design
baseline is commit `878e242` and its final report is under
`implementation-design/reviews/completed-design-synthesis.md`. Local development
does not read that repository or its environment files. No research snapshots,
third-party runtime code, or credentials have been imported here, and no project
license or publication permission is implied by creating this repository.
