# Food menu V2 protocol review

Reviewed 22 October 2026, Europe/Berlin. This is a static protocol and
contract review of the uncommitted Food V2 model increment. No shared Docker,
PostgreSQL, browser, or full-task check was run by this reviewer.

## Scope

The review covers:

- `ocp.food.menu` version `2.0.0` and its coexistence with version `1.0.0`;
- scheduled offers, availability windows, time zones, and quote expiry;
- size-specific extra prices, defaults, and immutable quoted terms;
- public projection and placement behavior; and
- the versioned OpenAPI components and focused unit/integration fixtures.

It does not establish an official UCP Food model, a general retail catalog, or
restaurant behavior in production.

## Findings and disposition

| Severity | Finding | Disposition |
| --- | --- | --- |
| High | Placement initially permitted a quote under one Food model revision to be accepted against the other model's current publication. That could apply V1 and V2 availability semantics inconsistently while retaining old price terms. | Resolved before this report. Placement now requires an exact `terms.Model == current.Model`, while same-model price-only revisions retain the original quoted price. |
| Medium | The first V2 validation accepted Go's special `Local` location even though the wire field promises an IANA business time zone. This would make offer availability depend on deployment configuration. | Resolved before this report. `Local` and empty names are rejected, UTC/IANA data are bundled, and the validator test covers the rejection. |
| Medium | Schedule precedence and quote-edge behavior were implemented but not described as contract semantics, leaving other clients unable to predict overlapping offers or quote validity. | Resolved before this report. The schema now states replacement-price behavior, priority then lexical-ID tie break, half-open windows, local/UTC boundary behavior, overnight split requirement, and edge-capped quote expiry. |
| Low | V2's `default_option_ids` initially carried the V1 validator's rules without the V1 reader-facing explanation that defaults are editable initial choices and must satisfy group limits. | Resolved before this report. The V2 schema now carries the corresponding default-choice semantics. |

## Static assessment

The registered V2 model is an independent, closed model version rather than an
unlabelled expansion of V1. V1's closed schema rejects V2 fields; V2 first
validates its own shape, then reduces to the V1 invariant set for shared
identifier, option, default, and selection checks. Public projection preserves
the V2 fields only for the V2 model and does not add them to a V1 document.

The pricing boundary is sound at static inspection:

- quotation takes its evaluation instant from PostgreSQL inside the transaction;
- selected item/variant windows and offers use that instant and the menu's
  declared business zone;
- an offer is a replacement base price, with variant-specific extra prices
  materialized only for the selected variant;
- active offer selection is deterministic; and
- the quote expires no later than its ordinary ten-minute limit or the next
  relevant schedule edge.

The response retains the evaluated base/effective base, selected offer ID,
extra amount, evaluation time, labels, and total. Placement rechecks current
selection/availability in the same model but intentionally retains the issued
price. That is an appropriate distinction between a live orderability check and
immutable agreed terms.

The source should retain the existing focused checks for DST repeated/skipped
wall times, precise UTC cutoff, invalid variant-price target, invalid zone,
model switching, same-model price revision, and idempotent placement replay.
The parent-run PostgreSQL and full checks remain necessary to establish the
transactional claims.

## Verdict

**Static acceptance pending the declared integration and full-task checks.**
No remaining protocol or contract blocker was found for the bounded Food V2
model. Do not call V2 UCP Food compatible or use it to imply that a schema
alone provides inventory, reservation, acceptance, payment, or fulfillment
behavior.
