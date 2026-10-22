# Food menu pricing version 2

Implementation decision, 22 October 2026. This is an owned Food model, not a UCP
Food specification or adoption of MenuConform's solver. The existing version 1
menu and its persisted quotes remain supported unchanged.
The runtime must install separate V1 and V2 validators/pricers concurrently;
never route retained V1 documents or quotes through V2 interpretation.

Restaurants need to sell differently sized dishes with appropriately priced
extras and to schedule offers such as a weekday lunch price. These belong to
the published menu model; customers select identifiers, never prices or discounts.

## Implemented contract

Use the exact model `ocp.food.menu` / `2.0.0` for the additional semantics.
Retain the current categories, translations, images, variants, availability,
option groups, defaults, and per-line notes. The API can expose all fields before
the merchant UI provides dedicated editing controls.

- Each variant retains its complete base `price_minor`, not an increment.
- An option retains a default `price_minor`. Optional `variant_prices` map variant
  identifiers of that item to replacement extra prices. Missing entries use the
  default. Reject unknown variants, negative amounts, and arithmetic overflow.
- A menu declares an IANA `time_zone`, validated by the backend. Each scheduled
  offer targets one item/variant and supplies a replacement base price, a stable
  offer identifier, an integer priority, and one or more weekly windows.
- Windows specify ISO weekdays (Monday = 1) and local `HH:MM` start/end. Start is
  inclusive and end exclusive. The first version requires start before end;
  represent overnight offers as two explicit windows. Optional UTC validity bounds
  constrain seasonal campaigns. Invalid zones, dates, references, and duplicate
  identifiers fail publication validation.
- Highest active priority wins. If priorities tie, the lexicographically smallest
  offer identifier wins. Offers do not stack, and do not alter extra prices.
  A replacement price may exceed the base price; this is scheduled pricing, not
  a promise that every offer is a discount.
- Optional item/variant `availability_windows` use the same weekly-window shape.
  Absence means no scheduled restriction; presence requires an active window.
  Item and variant eligibility combine with manual availability using AND.
  Public catalogs retain these dishes with their schedule, so clients can show
  when they return; quotes reject selections outside eligible windows.

Bound menus to 200 offers, each with at most 14 windows; item/variant availability
schedules also have at most 14 windows. Offer identifiers use `[a-z][a-z0-9_-]{0,63}`
and are unique within the menu; priorities are integers from 0 to 1,000. Weekday
arrays contain unique integers from 1 to 7. Require exact `HH:MM` values from
`00:00` through `23:59`; allow `24:00` only as an exclusive end to represent a
window ending at midnight. Optional `valid_from` and `valid_until` are RFC3339
UTC timestamps ending in `Z`, respectively inclusive and exclusive. If both
exist, start must precede end. Empty schedules and duplicate windows are invalid.

The pricing result retains the selected variant's original base, applied offer
identifier (if any), replacement base, selected extras' actual prices, evaluation
instant, and final totals. These facts explain the agreed amount without having
to reconstruct an old menu or apply today's schedule.

## Time and purchase boundaries

The server supplies the evaluation instant, not the customer. Weekly windows
are evaluated in business-local wall time. On the autumn DST repetition, both
occurrences of a matching local time qualify; nonexistent spring times simply
never occur. UTC campaign bounds distinguish instants where that is needed.

Quote expiry is no later than the earliest future price or availability boundary
across all selected lines, including currently inactive offers and UTC campaign
bounds. This
prevents a normal quote lifetime from extending stale pricing into a lunch
window. The existing expiry ceiling remains. The HTTP journey exercises V2
quotation and placement with retained pricing facts. Successful
placement retains agreed terms; it does not reprice an accepted order later.

Existing manual availability remains: no ingredient inventory is required.
Publication changes and placement retain their current store serialization and
revision/authority checks. A quote is neither a stock reservation nor acceptance.

## Verification

Unit and PostgreSQL/HTTP tests exercise size-specific extras, invalid references,
monetary limits, offer priority/ties, inclusive/exclusive boundaries, IANA zones,
DST changes, campaign bounds, quote expiry, retained prices after publication
changes, model mismatch, and replay. V1 remains supported; V2 is an explicit
installed model rather than a silent reinterpretation of V1 documents.
The latest completed Gremlins follow-up killed 151 Food mutants, with 10 surviving
and none uncovered. Survivors remain test-strength debt, not automatically
equivalent code. See `STATUS.md` for final regression evidence. The UI does not
yet expose every V2 field.
