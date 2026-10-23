# Contract and persistence foundation

21 October 2026, Europe/Berlin (UTC+02:00). This is the implementation handoff for
steps 1 and 2, not a claim that ordering or federation already runs here.

## What we are building next

The first maintained slice lets an authorized merchant create a store and retain
versioned draft offerings. It connects real HTTP contracts to PostgreSQL and tests
tenant and action boundaries. It does not publish to ONIX, issue purchase terms,
take payment, or implement a storefront. Those remain the next connected slices.

| Boundary | Foundation decision |
| --- | --- |
| Runtime | Owned Go implementation under `services/platform/`; retained ONIX integration under `services/onix/`; no replacement local-only network |
| Wire format | JSON request/response bodies; OpenAPI source in modular YAML; generated bundles are build artifacts, not editing sources |
| Contract ownership | Provider entry document composes account, offering, and operational paths; reusable parameters, responses, and resource schemas have their own files |
| Contract tooling | OpenAPI 3.0.3, oapi-codegen v2.6.0, runtime v1.4.1, and kin-openapi v0.133.0; generated Go types, HTTP interfaces, embedded contract, and client |
| Persistence | pgx v5.7.6 PostgreSQL access; embedded SQL migrations through Goose v3.26.0; explicit migration command, not automatic API-startup DDL |
| Authentication | Revocable, expiring opaque bearer grants stored as SHA-256 digests; merchant and action scope checked on every private operation; no public signup or production identity-provider claim |
| Data isolation | Runtime non-owner role, forced row-level security, transaction-local merchant context, composite tenant foreign keys; migration/bootstrap credentials separate from API credentials |
| Mutation safety | Required scoped replay key; complete typed request fingerprint; optimistic offering revision; commit mutation and replay result together |

OpenAPI 3.0.3 is an intentionally conservative generator target, not a conclusion
that 3.1 is unsuitable. Upgrade the pinned cohort when its generated clients,
validation, and drift checks pass. JSON Schema dialect claims apply only to their
named contracts; an OpenAPI schema is not automatically a standalone 2020-12 schema.
No Beckn/UCP wire compatibility is claimed for these owned administration APIs.

## Requirements carried forward from founder review

Offering is a module, not a fixed menu database or a second purchase engine.
Versioned offering models define items, variants, and supported combinations;
model-specific validators and editing/presentation support are replaceable.
The first model is a small Food menu. Its draft format must not become the format
of every future domain. Immutable revisions retain the exact model and version.
Unknown models fail explicitly until a compatible validator/handler is installed.
Future purchase terms retain their original model/contract meaning after upgrades.

Manual availability is the initial restaurant mode: staff can disable a product
without keeping inventory counts. It is not a guarantee of infinite stock.
Tracked stock/capacity can later add transactional allocation; ingredient-driven
availability is later support. Merchant fulfilment initially means preparation
for pickup or own delivery. External fulfilment needs its own authorized task and
progress contract rather than being silently treated as own delivery.

The addressing direction is a storefront hostname with `/api` and `/mcp` paths,
on provider subdomains and verified merchant domains. Clients use advertised
endpoint URLs, not assumptions about co-location. Foundation APIs bind to a
loopback development host and still require explicit merchant authorization;
custom-domain provisioning, DNS/TLS automation, browser login, and MCP routing
are not part of this slice.

## Scope and acceptance of step 2

- Validate all local references and contract examples; generate a usable Go client
  and server interface, and fail drift checks rather than edit generated files.
- Describe liveness plus create/read store and write/read immutable draft-offering
  revisions. Keep publication, purchases, customer accounts, and payment out of the
  executable contract until their implementations arrive.
- Exercise actual PostgreSQL migrations, repeated migration, authorized mutations,
  missing/revoked/expired/wrong-scope credentials, wrong-merchant access, concurrent
  revision conflicts, replay/conflict, and pooled tenant-context cleanup.
- Validate menu definitions structurally and semantically at the server boundary;
  a generated form is not the source of purchase authority. This slice validates
  catalog definitions, not customer purchase choices or full Food conformance.
- Keep Task and Docker as the local command interface. Add no CI, cloud resources,
  real merchant data, payment secrets, or native-network readiness claims.

The initial provisioning command is an administrative development operation, not
the Merchant account API's future signup flow. It creates test merchant identity,
staff grant, and one-time credential output without storing plaintext credentials.
Production authentication and public account onboarding require their own release
checks. No email login, OAuth, or universal customer account is implied.

## Tooling references

Checked 21 October 2026. These inform tool selection; module checksums and generated
artifact tests establish the specific cohort actually used.

- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen): generation, external references, and strict server interfaces.
- [Goose provider](https://pressly.github.io/goose/documentation/provider/): embedded migration files and explicit database drivers.
- [OpenAPI 3.0.3](https://spec.openapis.org/oas/v3.0.3.html): references, schemas, media types, and security declarations.

## Reviews and handoff

Updated 22 October 2026 by founder direction: generated code and error catalogs
are not committed. Source contracts, generator configuration, locked tools, and
handwritten implementations are tracked; Tasks and Docker generate outputs before
compilation. This supersedes the earlier review recommendation to commit Go output.
All transitive Go modules are pinned in
`go.mod`/`go.sum`. The runtime role is neither owner, superuser, nor BYPASSRLS.
Authentication uses a narrowly scoped, schema-qualified SECURITY DEFINER lookup
with fixed search path and no PUBLIC execution or runtime credential-table reads.
Tokens have 256 bits of random entropy. Mutation fingerprints are SHA-256 over
deterministic JSON of the fully decoded request and operation/resource context;
scope includes merchant, authenticated grant, operation, resource, and client key.
Successful replay status/body are stored atomically, with no automatic replay
expiry in this first slice. Replay does not bypass fresh authentication or scopes.
Deployment routes and custom hostnames are not tenant identifiers.

Protocol review constraints are accepted: the REST base is `/api/v1`. Optional
additions may remain in v1; removals or changed meaning require a new major
contract. Bearer grants carry `store:read`, `store:write`, `offering:read`, and/or
`offering:write`, bound to one merchant. Missing/invalid/revoked/expired grants
return 401; authenticated insufficient action or tenant authority returns 403.
Errors use reusable `application/problem+json` Problem Details (RFC 9457) with
stable machine codes and safe details. Mutations require `Idempotency-Key`;
matching successful retries return the original status/body, changed requests
return 409. Replay records are retained with the merchant data, with no automatic
expiry or key reuse in this foundation; bounded cleanup needs a future explicit
contract. Offering writes require `base_revision` (zero for the first revision),
and conflict returns 409 with `current_revision`. Use no second precondition scheme.

The source layout is `contracts/openapi/v1/openapi.yaml`, with `paths/`, bounded
`schemas/`, `components/`, `domains/food/menu/v1/`, and fictional validated
`examples/`. All references are reviewed local files. Committed generated Go lives
with its consuming HTTP adapter; ignored bundles live in `contracts/build/`.
Source field names use snake_case consistently. The modular source, generated
types, and actual round trips are checked together before claiming this cohort works.

Review disposition: all step-1 blockers are resolved as explicit choices above;
engineering/security assertions remain required executable acceptance in step 2.
Reports: [engineering](reviews/step-1-engineering.md) and
[protocol](reviews/step-1-protocol.md). No implementation or production acceptance
is inferred from these plan reviews.

Step 1 receives separate engineering and protocol reviews before step 2 proceeds.
Step 2 receives both perspectives against the actual code, contracts, and executed
checks, followed by corrections. Review reports identify snapshots and limitations;
automated reviews are not independent interoperability or production approval.

## Implementation corrections from step 2

The first compiler run exposed an incompatible runtime v1.1.2 pin; v1.4.1 supplies
the generated client's required styling helpers. The local bundler now gives
reusable resources, parameters, responses, and Food types intentional root names,
not generated filesystem/hash names. It reloads serialized output before accepting
the bundle. PascalCase operation IDs preserve identity through this generator's
normalization; JSON fields remain snake_case. The config's output setting overrides
the CLI flag, so drift checking uses a temporary derived config and never edits the
source artifact. A deliberate generated-comment change failed the check as expected
without being overwritten; the temporary change was then removed.

Both technical reviews also require matching 413/415 behavior, readable choice
labels, request correlation, explicit expected errors, and executable non-owner
checks. Protected operations declare `x-required-scope`, loaded and checked against
known actions at startup. Every response has a fresh `X-Request-ID`; Problem
`instance` carries its UUID URN. Runtime startup rejects explicit role memberships
and ownership/privileged-role reachability, not merely unsafe login flags.
The integration suite now asserts those negative controls and the credential
lookup function's owner, ACL, and fixed search path. Replay hashes explicitly include
operation/resource as well as the complete typed request. Read-only responses no
longer promise stored mutation replay. Review retention/archival before general
availability; automatic cleanup remains deliberately unimplemented.
