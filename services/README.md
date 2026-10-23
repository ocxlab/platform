# Backend runtimes

Directories here follow independently packaged runtime boundaries, not programming
languages or a requirement to deploy every logical service separately.

| Runtime | Responsibility | Current state |
| --- | --- | --- |
| `platform/` | Owned Go API and workers for core, Food, Shopping, Lodging, scoped payment, and discovery | Combined core/Food default; separately selectable domain processes and local tests |
| `onix/` | Retained native publication/crawling runtime and its plugin/configuration integration | Optional local network composition; see its README for scope |

The provider runtime's publication adapter sends public revisions through ONIX;
the native crawler feeds the owned compact discovery projection. Purchase
requests reach the authorized provider commerce service, not a discovery relay.
The selected UCP-compatible interfaces and reused helpers belong in the owned
runtime's adapters/modules; the plan does not require a second native UCP checkout
server or a competing purchase authority.

One provider runtime may expose several logical services and share domain code
between API and worker entry points. Payment still has its own contract and
credential/storage access boundary. If workload or isolation later warrants a
separate runtime, extract it along that boundary rather than creating mandatory
microservices in the scaffold. `platform/` identifies the owned software,
not an assumption that every OCP operator must use it. It keeps one Go module;
shared technical adapters live under `internal/infra/`, outside business modules.

Each runtime owns its Dockerfile and runtime-specific configuration. Local and
later production compositions belong under `deploy/`; the root Taskfile is the
developer command interface. Upstream runtime versions, plugins, notices, and
owned modifications must be recorded when adopted, not silently downloaded at
startup or copied from private research caches.
