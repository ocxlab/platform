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
task test:mutation:work
task test:mutation:mcp
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
- `reports/mutation/go/work.json` for the separate worker task.
- `reports/mutation/go/mcp.json` for merchant price/meal edits, meal reads, and diff rules.

Sandbox directories and reports are also excluded from Docker build context.

The worker task targets `core/work/scheduler.go` and `worker.go`, using unit fakes
in the same isolated-copy workflow. It excludes `store.go`, `stats.go`, and
`wake.go`, whose live database behavior is exercised by integration tests instead.
Gremlins matches these exclusions against basenames; the initial exploratory run
used a directory-prefixed expression and unintentionally included those adapters.
The corrected task's report lists only the two intended files. Architecture guards,
SQL functions, telemetry internals, and browser behavior are not mutated by this task.

## Executed results

24 October 2026. A focused Stryker run of `src/lib/basket-checkout.ts`
killed **87 of 92 mutations (94.57%)**, with five survivors and zero uncovered
expressions, timeouts, or errors. Its first run scored 75%; additional tests now
cover ready-input invalidation, placement-time input changes, stale failures,
explicit retries, expiry renewal, and initial context. Six actor tests exercise
serialized cookie-writing quotes, fail-closed placement, duplicate submission,
retained retry identity, and disposal. Run in `web-mutation` with
`pnpm exec stryker run --mutate src/lib/basket-checkout.ts`.

The survivors remove type metadata, make the CHANGE-only action's event guard
unconditional, change the unused machine ID, or change initial requested/error
values overwritten by idle entry. They appear equivalent under the current
wiring; this is not a proof for future callers. No mutants were suppressed.
The run does not mutate enhanced Svelte forms, browser redirects, or database
checkout. Those require separate browser and integration evidence.

24 October 2026. A focused Stryker run on the new XState-based
`src/lib/live-updates.ts` killed **111 of 114 mutations (97.37%)**, with three
survivors, zero uncovered expressions, timeouts, or errors. The survivors remove
type-only setup metadata or change the initial pending flag, which is overwritten
on entry to every read. No mutations were excluded to obtain this result. This
run added an assertion for an SSE reconnect during an in-flight read. It does not
mutate Svelte components or the TanStack adapter. Repeat it in the `web-mutation`
Docker service with `pnpm exec stryker run --mutate src/lib/live-updates.ts`;
the regular web mutation task now also includes this module. An initial attempt
in `web-tools` stopped because that lightweight image lacks `ps`; the dedicated
mutation image already supplies it and completed the run.

24 October 2026. The expanded Food adapter run (`task test:mutation:mcp`)
killed **193 mutations, with zero survivors and 197 uncovered**: 49.49% mutant
coverage, with no timeouts, nonviable mutations, or skips. Coverage collection
took 2.181 seconds and mutation execution 1.370 seconds, excluding task startup.
This now includes the Customer Food adapter's pure basket-revision rules. Most
new HTTP/SQL callbacks remain uncovered by this unit-only runner. Its passing
result is not whole-adapter coverage; PostgreSQL/HTTP integration runs are
separate evidence and are not counted as mutation kills.

23 October 2026. The expanded `task test:mutation:mcp` includes `meal_offers.go`
alongside `menu_edits.go`: **139 killed, zero surviving, 38 uncovered** mutations,
with no timeouts, nonviable mutations, or skips. Efficacy is 100% among exercised
mutations; mutant coverage is 78.53%. Mutation execution took 2.109 seconds after
3.889 seconds gathering coverage, excluding Docker/task startup. The 38 uncovered
expressions are schema/registration and HTTP/SQL callback paths. The real HTTP/SDK
and PostgreSQL suite exercises those paths separately, not through these mutants.
`merchant.go` and `menu_details.go` remain excluded. The
[retained summary](evidence/mcp-mutation-2026-10-23.csv) identifies this expanded scope;
the earlier narrower result below is not a whole-adapter comparison.

23 October 2026. `task test:mutation:mcp` targeted only `menu_edits.go` in the
Food merchant adapter: **75 killed, zero surviving, 25 uncovered** mutations;
100% efficacy among exercised mutations and 75% mutant coverage. No timeouts,
nonviable mutations, or skips were reported. Mutation execution took 1.447 seconds
after 4.401 seconds gathering coverage (not total Docker/task wall time).
The uncovered expressions belong to tool registration/schema refinement and
HTTP/SQL callbacks, which this unit-only mutation run does not execute. Real
HTTP/PostgreSQL integration tests exercise the tool path separately; passing
them does not mean those 25 mutations were killed. `merchant.go` and
`menu_details.go` are excluded; this is not whole-MCP mutation coverage.

23 October 2026. `task test:mutation:work` completed with **54 killed, four
surviving, and two uncovered** mutations: **93.10% efficacy** (54/58 exercised)
and **96.67% mutant coverage** (58/60). No timeouts, nonviable mutations, or skipped
mutations were reported. The run took about 113 seconds. The preceding same-scope run
was 53/58; adding an explicit empty-drain metric assertion killed that survivor.
Other new assertions cover valid concurrency/safety limits, wake spacing, unexpected
clean listener exit, lease/timeout lengths, stable identities, and attempt outcomes.

Four scheduler mutations survive:

| Location | Mutation | Assessment |
| --- | --- | --- |
| `scheduler.go:75` | Pause when delay is zero as well as positive. | Apparent timing-boundary equivalence: a zero-duration timer expires immediately. Not a general proof of equivalence under every scheduling interleaving. |
| `scheduler.go:88` | Snapshot only after strictly more than a minute, rather than at least a minute. | Exact elapsed-time boundary not asserted. The unit test checks rate limiting, not a full minute-long cadence. |
| `scheduler.go:108` | Change arithmetic in the five-second claim/finish-error fallback. | Exact fallback interval is a coverage gap: tests recover using a shorter safety interval, which caps either value. |
| `scheduler.go:117` | Change arithmetic in the five-second schedule-error fallback. | Same interval-assertion gap as the preceding row; error recovery itself is tested. |

The two uncovered expressions are the minimum-drain constant declaration and the
two-second cleanup timeout for a wake source that ignores cancellation. Existing
tests exercise cooperative cancellation, not that forced-wait boundary. These
gaps remain visible; they are not counted as killed or silently suppressed.
Unit tests cover reconnect delay growth/reset, but `wake.go` is outside this
mutation scope. SQL ACL/fencing and process-crash tests are separate evidence.

The earlier exploratory run's exclusion mismatch is documented above; its broader
totals are not compared with this final narrower scope. The machine-readable report
remains an ignored local artifact; [summary CSV](evidence/worker-mutation-2026-10-23.csv)
retains the final counts and explicit scope.

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

The 23 October merchant expansion ran `task test:mutation:mcp` against the Food
adapter's price/offer/diff helpers plus menu setup, publication selection digest,
and order detail projections. Gremlins v0.6.0 killed 180, left zero survivors,
and reported 120 uncovered mutants (60.00% mutant coverage), with no timeouts,
nonviable mutants, or skips. The mutation phase took 1.496 seconds after a
2.935-second coverage pass; command/container startup is additional. Registration
and HTTP/SQL callbacks account for most uncovered expressions. Real HTTP/SQL
integration checks cover those paths separately, but do not turn uncovered
mutants into killed ones. This run does not mutate Food application/persistence
transactions or prove model-driven task quality. The retained CSV records both
the earlier narrower scope and this expanded scope.

SQL persistence, HTTP handlers, Svelte components, and generated contracts are
not mutation-tested by these bounded tasks. Their normal database, component,
contract, and browser tests are separate. In particular, a 100% transition-rule
score does not establish correct transactions, authorization, or real payment
processor integration. Extend mutation scope deliberately as modules stabilize;
do not represent this run as whole-application coverage.
