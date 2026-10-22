# Food menu V2 engineering review

Reviewed 22 October 2026, Europe/Berlin (UTC+02:00), against the uncommitted
Food V2 implementation. This is a static engineering review. The implementation
agent owned the active PostgreSQL integration runner; this review did not run a
shared Docker, database, or browser task.

## Scope reviewed

- the V2 menu schema and concurrent V1/V2 registration;
- scheduled price and availability evaluation, including business time zones,
  UTC validity bounds, and quote-expiry boundaries;
- quote creation with database-derived evaluation time;
- placement validation and retained quoted terms; and
- quoted price-explanation contract fields and the focused unit/integration tests.

The review does not establish production restaurant behaviour, UI editing support,
or interoperability with an external Food standard.

## Findings and disposition

| Severity | Finding | Disposition |
| --- | --- | --- |
| High | Placement initially accepted a retained V2 quote after a merchant republished V1, and the reverse, because it checked only that each model was a supported Food model. It then applied current availability under different model semantics while retaining old terms. | Resolved. `PlaceFoodOrder` now requires exact `terms.Model == current.Model`; V2-to-V1 and V1-to-V2 republish/placement rejection cases are included in `integration/food_v2_test.go`. A price-only revision within the same model remains valid and retains the issued price. |
| Medium | Variant-specific extra pricing initially copied the complete menu through JSON once per selected line. This was unnecessarily expensive and hid the actual line-level pricing boundary. | Resolved. `PriceModel` now copies only the selected item before materializing its variant-specific option prices. The existing bounded line/item/option limits continue to cap work. |
| Low | Structurally identical windows whose weekday arrays were written in different orders could evade schema-level JSON uniqueness. | Resolved. V2 validation canonicalizes weekday order while checking semantic duplicates. |

## Static assessment

The V2 design maintains a meaningful boundary from V1:

- `ocp.food.menu` versions `1.0.0` and `2.0.0` are separately registered with
  their own validator entries, while persisted terms retain their exact model.
  V1 validation rejects V2 fields, and V2 reduces to the V1 invariants only after
  validating its additional fields.
- V2 pricing uses the timestamp obtained from PostgreSQL in the quote transaction,
  rather than a client clock. It uses embedded IANA timezone data, inclusive start
  and exclusive end semantics, and absolute UTC bounds. The focused tests cover
  the two repeated local times at the autumn DST transition, a spring-gap
  exclusion, and a sub-minute UTC cutoff.
- Item and variant schedules combine with the existing manual availability check;
  active offers choose highest priority and then lexicographically smallest offer
  ID. Per-variant option replacements are applied only to the selected variant.
- The returned deadline is conservative: it is no later than the ordinary
  ten-minute lifetime and is shortened at relevant minute or UTC validity edges.
  Conservative expiry at an individual window transition is acceptable even if
  another overlapping rule leaves the effective price unchanged.
- The quote retains the server-evaluated base/effective base, chosen offer ID,
  extras price, timestamp, display labels, and total. Placement rechecks present
  availability/selection under the same model but does not recalculate and replace
  the retained agreed price.

## Small follow-up

`PriceModel` still repeats the `len(lines)` range check after completing line
pricing. The early check is sufficient; removing the second one would simplify
the function but is not a correctness or release blocker.

## Verification boundary and verdict

The implementation agent reported Food and HTTP API unit tests passing and was
running PostgreSQL integration tests. Those results are not reviewer-executed
evidence in this report. The pending integration run should specifically confirm
the added model-switch rejection paths, same-model price-only retention, and
transactional server-time quote flow.

**Static acceptance, pending the declared PostgreSQL and full-task verification.**
No remaining source-level blocker was identified for the bounded V2 model.
