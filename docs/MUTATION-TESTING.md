# Mutation testing

Mutation testing deliberately changes program expressions and asks whether tests
notice. It exposed missing boundary and cookie assertions in this batch; those
tests now exercise the intended behavior rather than only happy paths. The scores
below cover the named modules, not the entire platform or its security.

## Repeat the runs

Run Docker and use the root Task interface:

```sh
task test:mutation:web
task test:mutation:go
```

The web runner pins Stryker core/Vitest runner **10.0.0** and uses a dedicated
Node-only Vitest configuration for pure TypeScript rules. It explicitly carries
SvelteKit's generated TypeScript config into the mutation sandbox. The additional
Docker tooling stage supplies `ps`, which Stryker's worker reaper needs; it does
not add that package to the application runtime. The runner fails below a 90%
mutation score. The ordinary full checks remain `task check` and
`task test:browser`; mutation runs are separate because they are slower.

The Go runner pins Gremlins **v0.6.0** and uses two workers. It mutates a temporary
copy of the owned Go module, never the source served by the running application.
No environment files, processor keys, or research source snapshots are copied.

Reports are local ignored artifacts:

- `apps/web/reports/mutation/mutation.html` and `mutation.json`;
- `reports/mutation/go/payment.json` and `food.json`.

Sandbox directories and reports are also excluded from Docker build context.

## Executed results

22 October 2026, Europe/Berlin (UTC+02:00). After adding pickup-action eligibility,
the same four-module Stryker scope killed **188/189 (99.47%)** mutants, with one
session-cleanup survivor and no uncovered mutants, timeouts, or errors. Order
eligibility now kills 88/88 mutants, including 29 for pickup-action guards.
Gremlins was rerun: payment rules remain 6/6 killed and Food 52/55 killed, with
no uncovered mutants. The survivor analysis below is unchanged. These runs do
not mutate the new PostgreSQL pickup transitions, HTML form/origin helpers, or Svelte
components; integration, parser unit, and browser tests cover those separately.

21 October 2026, Europe/Berlin (UTC+02:00). The final completed runs produced:

| Mutation scope | Killed | Survived | No coverage | Result |
| --- | ---: | ---: | ---: | --- |
| TS order-action eligibility | 59 | 0 | 0 | 100% |
| TS EUR formatting | 6 | 0 | 0 | 100% |
| TS guest cookies and checkout intent | 43 | 0 | 0 | 100% |
| TS development sessions | 51 | 1 | 0 | 98.08% |
| Go development-payment transition rules | 6 | 0 | 0 | 100% efficacy |
| Go Food validation, projection, and pricing | 52 | 3 | 0 | 94.55% efficacy |

Stryker's aggregate is **159/160 (99.38%)**, without timeouts or errors. Its first
successful run was 118/160 (73.75%), with 36 survivors and six uncovered mutants.
Added tests cover expiry boundaries, explicit insecure-cookie opt-in, scoped
deletion, malformed intents, revoked/replaced handles, and session-capacity cleanup.

Gremlins' first Food run killed 27, left 16 alive, and reported 12 uncovered.
Added tests cover exact monetary/quantity/line/option boundaries, invalid menus,
duplicate names/identifiers, and schema validation. The final Go mutation runs
report no uncovered mutants in their selected packages. These are each tool's
generated mutation sets, not every possible defect.

## Remaining survivors

One TypeScript mutant unconditionally calls `Map.delete` in `disconnect` even
when the cookie is absent. Deleting `undefined` from this string-keyed map is a
no-op; this appears behaviorally equivalent. It remains visible in the report.

The three Go survivors alter redundant arithmetic guards in
`internal/food/offering/selection.go`: two affect `MaxTotal-unit`, and one replaces
`MaxTotal/quantity` with multiplication. Later line/aggregate limits reject the
same excessive totals under the validated, bounded inputs. This is a qualitative
equivalence assessment, not a proof for arbitrary inputs or future changes.
Do not remove useful guards or suppress these mutants merely to obtain 100%.

## Limits

SQL persistence, HTTP handlers, Svelte components, and generated contracts are
not mutation-tested by these bounded tasks. Their normal database, component,
contract, and browser tests are separate. In particular, a 100% transition-rule
score does not establish correct transactions, authorization, or real payment
processor integration. Extend mutation scope deliberately as modules stabilize;
do not represent this run as whole-application coverage.
