# Three-domain implementation acceptance

22 October 2026. The agreed pre-administration slice now has Food V2, internal
Shopping and Lodging APIs, native ONIX Food discovery, and selectively deployable
API/worker roles. Its purpose is to test the shared foundation against three
different kinds of commerce, not to launch three customer products at once.

## What is available

| Area | Implemented behavior | Evidence entry point |
| --- | --- | --- |
| Food V2 | Size-specific extra prices, scheduled offers, availability windows, retained quote terms and boundary-aware expiry; existing categories, translations, defaults and images remain supported | `FOOD-PRICING-V2.md` |
| Shopping | Catalog administration, checkout creation/read/update/cancel/complete, authoritative stock allocation and retained orders | `INTERNAL-COMMERCE.md` |
| Lodging | Property/unit/rate catalogs, booking creation/read/update/cancel/complete, dated physical-unit allocation and retained confirmations | `INTERNAL-COMMERCE.md` |
| Shared foundation | Scoped identity admission, typed payment subjects, atomic outstanding pay-later obligations, immutable delivery intents and bounded leased workers | `DOMAIN-BOUNDARIES.md` |
| Network | Native publishers/crawler carry signed Food projections between two configured providers; discovery routes to each provider's authoritative quote API | `NETWORK-ACCEPTANCE.md` |

The Food V2 contract is owned and MenuConform-informed; it is not an upstream
MenuConform runtime or an official UCP Food specification. Shopping and Lodging
implement selected pinned UCP draft interactions. They are internal only, have
no customer UI, and use explicit outstanding pay-later obligations rather than
card-payment success. The public Food listener does not expose their routes.

## Verification boundaries

`task check` covers generation, source-only tracking, error catalogs, formatting,
OpenAPI consistency, Spectral, Go vet/race tests, PostgreSQL integration, frontend
tests and image builds. `task test:domains` additionally builds the maintained
binary and runs separate API/worker processes with pinned upstream response
schema checks. `task test:network` uses actual ONIX containers, not a rewritten
publisher or crawler. Final command outcomes are recorded in `STATUS.md`.

The domain suite exercises credential and store/customer/domain isolation,
invalid transport, canonical replay, stale pricing, concurrent stock allocation,
Lodging overlap and adjacency, database exclusion enforcement, session expiry,
and public-route denial. Process tests kill an API and replay through its replica,
saturate Shopping while Lodging still answers, and kill a worker after an external
receiver retains an effect but before acknowledging it. Recovery repeats the same
catalog identity. That receiver is a test double; native delivery is covered by
the separate ONIX journey.

The same Docker image has actually run the public Food API, separate internal
Shopping and Lodging APIs, and a catalog worker. Internal roles expose no host
ports. Their configured local budget is 128 MiB / 0.5 CPU each, with bounded
database pools. A quiet sample measured about 6.8 MiB per internal API and
4.6 MiB for the worker. This proves role packaging and bounded local behavior,
not production throughput, cloud cost, independent database capacity or failover.

Engineering and protocol reviewers inspected the implementation and closed their
identified blockers. Their reviews were static; execution evidence belongs to
the command runs above, not independent reviewer test runs.

## Explicit remaining limits

- Upstream UCP checks target draft source commit
  `d13cc61455d2a90f7e6038df07fdbe72dcccdcc7`, wire version `2026-08-25`.
  They validate actual session responses and a failing negative control, not
  whole-protocol conformance or an independent client product.
- Internal authorization is a fixed, enrolled platform cohort, not general UCP
  profile discovery or public OAuth admission. Post-confirmation cancellations,
  returns, refunds and real Shopping/Lodging payment adapters remain outside it.
- ONIX enrollment and registry origins are controlled fixtures. Public trust,
  key rotation and active freshness renewal for unchanged catalogs remain release
  work. A signed hidden projection implements withdrawal; native catalog
  retirement and Beckn checkout are not claimed.
- PostgreSQL remains a shared capacity/failure boundary. Moving a domain to its
  own database needs identity/payment integration and migration, not just a flag.
- Food's Gremlins run killed 151 mutants, with 10 surviving and none uncovered.
  Capability killed 5/5 and payment 17/17. Survivors are recorded below as test
  strength debt, not assumed equivalent. No new Stryker run was needed for this
  backend/contract slice; no frontend logic was changed.

### Food mutation survivors

Locations describe this checkpoint, not permanent identifiers. JSON reports under
ignored `reports/mutation/go` are reproducible through `task test:mutation:go`.

| File | Line:column | Mutation |
| --- | --- | --- |
| `scheduled.go` | 118:22 | CONDITIONALS_BOUNDARY |
| `scheduled.go` | 118:45 | CONDITIONALS_BOUNDARY |
| `scheduled.go` | 192:34 | CONDITIONALS_BOUNDARY |
| `scheduled.go` | 234:146 | CONDITIONALS_BOUNDARY |
| `scheduled.go` | 280:27 | CONDITIONALS_BOUNDARY |
| `scheduled.go` | 280:37 | INVERT_NEGATIVES |
| `scheduled.go` | 280:37 | ARITHMETIC_BASE |
| `selection.go` | 103:54 | INVERT_NEGATIVES |
| `selection.go` | 103:54 | ARITHMETIC_BASE |
| `selection.go` | 124:21 | ARITHMETIC_BASE |

Provider-admin redesign has deliberately not begun. It can now build on actual
domain capabilities and bounded HTTP/work telemetry. Ask for the founder's
inspiration screenshots before starting that visual work.
