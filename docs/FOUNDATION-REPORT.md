# Store and offering foundation

21 October 2026, Europe/Berlin (UTC+02:00).

The first maintained business slice connects a merchant-authorized JSON API to
PostgreSQL. A merchant can create a store, read it, and retain immutable revisions
of a draft offering. The first offering model is a small Food menu with variants,
named option groups/extras, and merchant-controlled availability. These are usable
administration operations, not placeholders for an ordering product.

## The two completed steps

Step 1 fixed the implementation boundary, tooling cohort, and acceptance rules
in [FOUNDATION-DECISIONS.md](FOUNDATION-DECISIONS.md). Engineering and protocol
reviews were incorporated before implementation. The handoff was committed
separately as `c5a710c`.

Step 2 implements that boundary with modular OpenAPI source, generated Go models,
HTTP routing and client, embedded SQL migrations, and actual database integration.
Read [the contracts guide](../contracts/README.md) for the source layout and
maintenance commands. Reusable resource schemas, headers, parameters, security,
responses, and domain definitions have explicit owning files. The generated client
uses intentional names such as `Store`, `OfferingRevision`, and `FoodOption`.

| Operation | Result |
| --- | --- |
| `POST /api/v1/merchants/{merchant_id}/stores` | Create a store under the authorized merchant |
| `GET /api/v1/merchants/{merchant_id}/stores/{store_id}` | Read that store |
| `PUT .../stores/{store_id}/offering` | Append a validated draft revision if `base_revision` matches |
| `GET .../stores/{store_id}/offering` | Read the current draft head |
| `GET .../stores/{store_id}/offering/revisions/{revision}` | Read a retained revision |
| `GET /health/live` | Establish process liveness, not commerce readiness |

## What the checks establish

`task check` validates modular and bundled OpenAPI, checks generated-code drift
without changing its source, runs formatting/vet/race checks, exercises PostgreSQL
integration, and builds the non-root API image. `task smoke` explicitly prepares
the development database, starts the built runtime, checks HTTP liveness and SQL,
and verifies that an unissued but well-shaped credential reaches the database-backed
lookup and is rejected. It creates no development merchant/store fixtures.

The PostgreSQL tests use a disposable service and a uniquely named database per
run. They apply migrations twice, then use the restricted runtime role through real
HTTP handlers. Executed behavior includes a generated-client round trip, scope and
tenant rejection, expired/revoked credentials, request validation and size/media
errors, unknown/invalid offering models, concurrent revision conflicts, concurrent
same-key retries producing one mutation, byte-identical successful replay, atomic
rollback, immutable historical revisions, and handler restart against retained data.

Database negative controls deliberately give the test runtime role table ownership
or administrative membership, verify startup rejects it, and restore the original
state. Other checks assert forced RLS, pooled context cleanup after commit/rollback,
no direct credential-table reads, and the credential function's owner, EXECUTE ACL,
and fixed search path. This is defense in depth for the application's tenant checks,
not a claim that arbitrary SQL execution by a compromised process is safe.

A deliberate generated-comment change made `task contracts:check` fail as expected;
before/after checksums confirmed the check had not overwritten that source. The
temporary change was removed, and normal generation comparison passed again.

## Review corrections worth retaining

Engineering and protocol reviews after each step are under [reviews/](reviews/).
The step-2 corrections include explicit non-owner/elevation checks, tested credential
lookup privileges, complete operation/resource/request fingerprints, readable extra
labels, correct `413`/`415` responses, fresh request identifiers, declared expected
error statuses, contract-owned action scopes, and stable generated names. Read-only
operations no longer make mutation-replay promises. The final dispositions are in
[the engineering review](reviews/step-2-engineering.md) and
[the protocol review](reviews/step-2-protocol.md).

The pinned cohort required two tooling corrections: a compatible oapi-codegen runtime
dependency and a bounded local-reference bundler workaround. Both are documented
in the contracts guide and protected by executable checks. No custom wire format
or runtime schema execution was introduced to work around the tools.

## Boundary and next slice

This foundation does not publish catalogs, issue agreed terms, place orders, reserve
stock, run native ONIX, prove UCP compatibility, or take payment. Nor does it test
storage-loss recovery, merchant migration, public security, or production login.
The development provisioning command creates temporary synthetic merchant grants;
it is not a public signup flow. The checks establish the bounded foundation, not
production readiness or independent interoperability.

The next implementation slice connects restaurant publication and a first purchase
journey under [IMPLEMENTATION-KICKOFF.md](IMPLEMENTATION-KICKOFF.md). Native network
integration and separately scoped payment remain explicit milestones. They have not
been replaced by a local-only storefront design. Do not start them merely to enlarge
this completed batch: agree the next bounded acceptance path first.
