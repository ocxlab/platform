# API contracts

Supported business REST APIs will be defined here before their handlers are built.
The foundation currently exposes process liveness only; it does not claim a
complete API contract or generated business implementation.

Keep four application surfaces distinct: Merchant commerce, Customer commerce,
Merchant account, and Customer account. Add identity, discovery, and payment
service contracts with their own owners. These are interface boundaries, not a
requirement for separate binaries or servers.

For the first business slice, choose an OpenAPI version and generator together,
pin their versions, and expose generation/validation/drift checks through Task
and Docker. Generated files identify their source and must be reproducible.
Examples must cover authorization, replay/conflicts, current agreed terms, and
required-action/error results as well as success. The initial operational endpoint
also needs a machine-readable description when that contract pipeline is introduced.

Do not import a whole UCP/Beckn schema family or claim compatibility merely to fill
this directory. Record exact adopted capabilities, projections, source versions,
and material-specific rights. Food has its own versioned behavior; future Shopping
and Lodging adapters invoke their appropriate handlers and shared execution.
