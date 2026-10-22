# Food menus and meal deals

23 October 2026. The newer Food implementation is the sole menu model, named
`ocp.food.menu` / `1.0.0` for this pre-release product. There is no parallel V2
validator, pricing engine, or API. Scheduled prices and availability were removed
at the founder's request. This coordinated draft change does not promise
compatibility with earlier generated clients or claim official UCP Food support.

Restaurants keep one catalog of dishes, drinks and sides. Categories, images,
DE/EN/AR names, full-price sizes, choice groups, defaults, notes and manual
availability remain available. Extras add their default `price_minor`; optional
`variant_prices` replace that amount for named sizes of the same dish. A missing
entry uses the default; an explicit zero is free. The merchant editor discloses
these overrides under **Prices by size**.

## A meal is an explicit purchase choice

A menu's optional `meal_deals` contain eligible `main_item_ids`, an additional
`surcharge_minor`, and allowed `addon_choices` of existing item/variant pairs.
The customer chooses a meal in dish customization, then its included drink or
side. The browser does not guess deals from unrelated basket lines.

For example, the demo offers döner or wraps with a drink for an additional €1,
and a Healthy Pizza meal with a salad for an additional €2. These are editable
fixture examples, not fixed platform prices. The main's size and extras keep
their normal prices; the included item's normal standalone price is not added.
Each main unit includes one selected addon: two identical meals mean two mains
and two drinks or sides. Different addon choices require separate basket lines.
There is one deal per line, without stacking or nested deals.

Included items cannot require customization. Optional extras, including defaults,
are not applied to them. Merchant help states this limitation. The API rejects
unknown references, duplicate identifiers/choices, invalid amounts and unknown
size overrides. Removing a dish in the editor prunes its deal references and
removes deals that have no eligible mains or included choices left.

## Quotes retain what the customer chose

Selection requests send only identifiers, quantities, selected options and notes;
they cannot dictate prices. The backend resolves the current publication and
checks both the main and included item's availability. Quotes retain localized
deal and addon labels, selected-size extra prices, addon quantity, surcharge,
and calculated totals. Merchant and customer order views display those same facts.

Placement rechecks the current menu's eligibility and availability without
replacing the issued price. A removed deal, changed membership or unavailable
included item cannot be ordered from an older quote. A price-only publication
does not silently reprice it. Quotes last ten minutes, are not reservations,
and neither accept the order nor trigger payment or kitchen work.

## Local fixture and data boundaries

`task dev:seed-media` adds the example deals and large-size cheese prices to
matching fictional products, along with the existing photos/translations. It
preserves existing deal definitions and does not restore deliberately removed
deals on subsequent runs. No store reset is required.

Before removing V2, the development database was inspected: all nine saved menu
revisions were labeled `1.0.0`, with no V2 quotes or orders. No persisted document
or agreed terms were rewritten. Existing V1 records remain usable. This finding
describes this workspace, not a migration guarantee for an independently deployed
older database. Generate contracts and rebuild consumers together.

See `STATUS.md` for executed checks and their limits. The contract is owned and
MenuConform-informed; no upstream MenuConform implementation is embedded.
