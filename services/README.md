# Backend runtimes

Directories here follow independently packaged runtime boundaries, not programming
languages or a requirement to deploy every logical service separately.

| Runtime | Responsibility | Current state |
| --- | --- | --- |
| `provider/` | Owned Go API and worker implementation for account, offering, commerce, identity, scoped payment, and compact discovery modules | API liveness scaffold only; workers and business modules not implemented |
| `onix/` | Retained native publication/crawling runtime and its plugin/configuration integration | Integration boundary documented; not yet imported or running here |

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
microservices in the scaffold. A directory named `provider` is this software
runtime, not an assumption that every OCP operator must use it.

Each runtime owns its Dockerfile and runtime-specific configuration. Local and
later production compositions belong under `deploy/`; the root Taskfile is the
developer command interface. Upstream runtime versions, plugins, notices, and
owned modifications must be recorded when adopted, not silently downloaded at
startup or copied from private research caches.
