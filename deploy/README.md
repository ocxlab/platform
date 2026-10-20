# Runtime composition

`local/compose.yaml` combines the current owned API host, its development toolchain,
and PostgreSQL. Call it through the root Taskfile. The explicit project directory
keeps build contexts, bind mounts, and optional `.env` resolution at the monorepo
root even though the Compose file lives here.

Runtime-specific Dockerfiles live with their source under `services/`.
The retained ONIX runtime will join this composition when its adoption/configuration
is implemented and tested. Its directory alone is not evidence of a running
network path. No cloud deployment, replica topology, CI provider, or production
secrets are selected here.
