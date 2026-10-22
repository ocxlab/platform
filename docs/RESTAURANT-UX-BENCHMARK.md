# Restaurant ordering UX research

Bounded public-document review, 22 October 2026, Europe/Berlin (UTC+02:00).
This note informs the ordering-controls and cart slices. It does not prescribe a
pixel-for-pixel copy or claim that a documented interface has been usability-tested
with our merchants. No partner account was accessed and no order was submitted to
another business. The current app is still a development workflow, not pilot-ready.

## What the public guides establish

| ID | Source and precise locator | Documented pattern | Consequence for our design |
| --- | --- | --- | --- |
| UX01 | [Wolt menu management](https://merchant.wolt.com/en/pol/learning-center/manage-menu), “How to edit menu item in the tableview,” “How to add options,” and “How to mark an item out of stock” | Searchable item tables, direct price/availability edits, detail panels, single/multiple choices | Give merchants a compact list and focused item editor; do not open every variant and extras field for every product. |
| UX02 | [Wolt opening hours](https://merchant.wolt.com/en/pol/learning-center/set-venue-opening-hours), regular and special hours sections | Weekly hours and exceptional closures are separate; live venue closure is also available | Separate a recurring schedule from “pause new orders now.” Show the current effective state. |
| UX03 | [DoorDash availability](https://help.doordash.com/en-us/merchants/article/what-is-my-store-status), “If you select Busy” and “If you select Pause” | Busy time adds preparation time; pause has duration/channel controls | Start with explicit pause/resume and a visible end time. Do not confuse pause with deleting a menu or cancelling accepted orders. Busy-time estimates are a later addition, not a prerequisite. |
| UX04 | [Deliveroo scheduled menus](https://help.deliveroo.com/en/articles/6059138-how-do-i-schedule-different-menus), default/scheduled menu definitions and publishing | Menu-category schedules differ from venue opening; edits are published deliberately | Preserve our draft/publication boundary. Do not use menu publication as the only mechanism for temporary operating status. Multiple scheduled menus can follow later. |
| UX05 | [Wolt developer menu guide](https://developer.wolt.com/docs/menuguide), “Menu Components” and item/option update sections | Items, categories, and reusable options; availability updates can be smaller than full-menu updates | Categories and reusable modifier groups merit explicit design. A fast sold-out control should not require re-entering a menu. Wolt's guide describes asynchronous processing, not our implementation. |
| UX06 | [DoorDash merchant cancellation](https://developer.doordash.com/en-US/docs/marketplace/how_to/merchant_order_cancellation/), overview and request fields | Some POS merchants auto-confirm validated orders; later operational cancellation is separately supported | Auto-accept is a merchant operating policy, not evidence that a kitchen started work. Keep acceptance, payment, fulfilment, and support distinct. The page contains an old-looking modification label; do not generalize its availability to all accounts/markets. |
| UX07 | [STAR Kebap & Pizzahaus public menu](https://www.stardoenerundpizzahaus.de/en/menu), category headings, items 21/22/73, Pizza Steinofen heading, drinks heading | A real local menu has sections, descriptions, fixed/base prices, a size statement, and a deposit note | A long flat product selector is not a credible final browsing interface. A base price marked “From” does not disclose all purchasable configurations. |

Wolt's guide also distinguishes POS-owned menus from direct portal editing. That
is a source-of-truth issue, not a reason to give our own UI and APIs contradictory
capabilities. They should use the same authorized operations and concurrency rules.

Lieferando's Austrian Partner Hub page was located, but retrieval returned no
usable body. Uber's menu help appeared in search results, but direct retrieval
failed with 404. Neither is used as detailed interface evidence here. These are
access limitations, not findings that either service lacks a feature. The English
Wolt guides describe a Polish locale; validate German product availability before
making competitive claims. Public guides are not authenticated UI observations.

## Additional customer UX evidence

Retrieved 22 October 2026, Europe/Berlin. These sources supplement the merchant
guides; they are not observations of our application:

| ID | Source and locator | Finding and design implication |
| --- | --- | --- |
| UX08 | [Nielsen, Progressive Disclosure](https://www.nngroup.com/articles/progressive-disclosure/), 3 December 2006, “Usability Criteria” and “Staged Disclosure” | Keep frequently needed information visible. Reveal a selected dish's configuration without turning each extra into a separate wizard step. Foundational guidance, not a new restaurant trial. |
| UX09 | [Wang, Four Principles to Reduce Cognitive Load](https://www.nngroup.com/articles/4-principles-reduce-cognitive-load/), 18 July 2025, “Structure,” “Transparency,” and “Mark Required Fields” | Logical grouping and explicit required/optional labels reduce ambiguity. Put required size/sauce choices first and label extras optional, with limits and surcharges beside choices. |
| UX10 | [Baymard food delivery benchmark 2026](https://baymard.com/research-articles/food-delivery-and-takeout-ux-benchmark-2026), paragraphs beginning “Dietary and customization needs” and “Category-specific filters” | The public benchmark identifies missing customization descriptions and dietary support as problems. Keep ingredient/allergen information discoverable; photos cannot substitute for it. No premium guidelines were accessed. |
| UX11 | [Scott, Grocery Add to Cart Buttons](https://baymard.com/research-articles/grocery-add-to-cart-buttons), 12 September 2023, “How Updating the Add To Cart Button Streamlines the Shopping Process” | Grocery testing supports immediate quantity feedback for directly addable products. Consider quick-add only for fixed-configuration items; grocery observations do not establish the best restaurant configuration flow. |

## A simple customer journey

Use a categorized menu with category navigation and optional within-menu search.
Following founder feedback, provide a responsive **photo-tile grid and a compact
list view** over the same product data. Both show names, prices or honest starting
prices, and availability; short descriptions and photos support recognition.
The view switch changes presentation, not configuration, cart contents, or pricing.
Products without photos need an intentional text-first layout, not large empty
image placeholders. Photos are optional for setup, not a prerequisite for ordering.

Selecting a product opens its configuration view: size, required choices,
optional extras, quantity, and the resulting price. Use radio controls for exactly
one choice and checkboxes for multiple choices. Required choices and surcharges
stay visible; progressive disclosure is not a license to hide conditions. Preserve
keyboard focus and a usable narrow-screen/RTL layout. A sheet/dialog is one possible
presentation; an ordinary detail page is an acceptable, simpler first implementation.

### Progressive disclosure without extra work

The proposed default is **browse → select a dish → configure → add to cart**.
The newest public material examined, Baymard's 2026 food-delivery benchmark, does
not establish a replacement for that familiar pattern. It reinforces the need to
make customization and dietary information discoverable. Our concrete proposal is:

1. On the menu, show the dish name, a short useful description, price or honest
   starting price, availability, and an optional photo. Category navigation and
   the grid/list switch remain outside individual item configuration.
2. Selecting the dish opens one focused configuration view. Show required choices
   first, such as pizza size or a mandatory sauce. Use explicit labels such as
   “Required · choose one” rather than making customers discover missing choices
   only after pressing Add. Do not assume an unselected required choice.
3. Show extras together as an optional group with prices, free options, and limits.
   Paid extras start unselected; skipping extras must remain a complete choice.
   A short extras group can stay expanded; collapse a genuinely long secondary
   group behind an obvious “Extras (optional)” control. Avoid a separate screen
   for every ingredient. Keep allergen/ingredient details easy to reach, without
   claiming that an ingredient omission guarantees allergy-safe preparation.
4. Keep quantity, the configured item total, and an “Add to cart” action easy to
   find. Explain an incomplete selection beside its group. Closing the view
   without adding must not quietly add an unfinished item.
5. Confirm the addition and let customers continue browsing. The cart preserves
   and displays selected size/extras and allows editing without starting again.

A bottled drink with one fixed variant and no choices may have direct quick-add;
its details remain accessible. A configurable pizza opens configuration instead.
Repeated additions of different configurations must not silently increment an
arbitrary existing line. Start with the common select/configure interaction;
quick-add is a convenience to evaluate, not another mandatory control on every tile.

For mobile, evaluate a full-height sheet or detail page rather than a tiny scrolling
dialog. If a dialog is used, manage focus, Escape/close, return focus, and accessible
naming. The benefit is less irrelevant information at each point, not the fewest
clicks at any cost. Validate this using the 48-product fixture: find a dish, add
paid/free extras, skip extras, add a differently configured second pizza, and edit
the cart. Repeat on narrow screens and Arabic RTL. No customer usability study
has yet been run.

The cart then shows editable configured lines, quantities, and a visible total.
The same product with different extras can occupy different lines. A server quote
confirms the current terms before placement; a cart is not itself an accepted
purchase. Expired quotes, price changes, and sold-out products must preserve the
customer's selections and explain what needs attention. These are our proposed
interaction decisions, not a claim that all reviewed services behave identically.

## A simple restaurant workspace

Keep Menu and Orders as the main working areas. A compact, always-visible order
status control belongs near Orders: accepting orders, paused until a stated time,
or closed by schedule. Put weekly hours and manual/automatic acceptance in store
settings, with a clear explanation of their effect on new orders. Do not scatter
ordinary daily controls among advanced configuration panels.

For menu management, show search, sections, item names, prices, and availability
first. Reveal sizes, extras, and selection rules only for the item being edited.
Shared extras should eventually be reusable without repeated manual entry. Keep
save/publish state explicit. Temporary sold-out status needs an immediate,
authorized operation with defined quote/placement consequences; designing that
operation is part of the next slice, not something the seed silently implements.

## Use a larger fixture as a reality check

`task dev:seed` creates **Marktplatz Küche (Demo)** with 48 fictional products in
its own merchant account. The checked-in fixture records seven intended sections,
sizes, required choices, paid/free extras, and one sold-out dessert. Prices and
modifier rules are synthetic; they are not attributed to STAR or any real merchant.
Repeat seeding never restores a deliberately withdrawn menu or overwrites edits.

The public STAR menu provides a separate, small acceptance probe. The installed
model can express a €4 portion of Pommes, a €4 portion of Bulgur, and a 30 cm
Margherita with a €9 starting price. The test checks only these data shapes; it
does not establish that the base-price pizza is the restaurant's final checkout
price. Source locators are the Beilagen items 21/22, Pizza Steinofen heading, and
item 73, retrieved on the date above. No whole menu, photo set, or customer data
has been copied. Generic names and three price facts are the limited test inputs.

The comparison reveals current model gaps:

- Categories and descriptions cannot round-trip through the closed v1 schema.
  Intended fixture sections remain metadata, not fake category prefixes in names.
- Photos, allergen/additive declarations, and drink deposit breakdowns need explicit
  design before a real German restaurant pilot. This review does not determine
  legal sufficiency or infer allergens from dish names.
- Existing per-item choices do not describe cross-choice constraints, size-dependent
  extra prices, reusable modifier definitions, or meal bundles. Do not flatten such
  cases silently; add them only when an actual menu requires them and test pricing.
- Menu hours, fulfillment-specific availability, and delivery-area rules are not
  product inventory. Keep those responsibilities separate.

Passing the fixture means supported configurations validate and price correctly;
it does not mean the whole real menu is losslessly modeled or our UI is easy to use.
Browser coverage loads all 48 items and exercises one configured purchase. Before
pilot acceptance, walk through a merchant-approved menu with known option rules,
then test finding, editing, sold-out handling, cart changes, and checkout on mobile.

## Retrieval trail

Search service: web search; no date filter; English and German terms; first returned
result batches only, no pagination. Three batches used these exact query groups:

1. `site.merchant.wolt.com menu editor options categories pause orders opening hours`;
   `site.merchants.deliveroo.com menu manager modifier groups busy mode`;
   `site.merchants.ubereats.com menu maker categories modifiers pause orders`.
2. `site.explore.wolt.com merchants manage menu options opening hours pause orders`;
   `site.merchant.wolt.com "pause" "orders"`;
   `site.partner.lieferando.de "Speisekarte" "Öffnungszeiten"`;
   `site.wolt.com/en/deu/karlsruhe/restaurant döner menu`.
3. `site.merchant.wolt.com/en "edit" "menu" "categories"`;
   `site.help.uber.com/merchants-and-restaurants "auto accept"`;
   `site.help.doordash.com/merchants "automatically accept" orders`;
   `site.lieferando.de/partner "Speisekarte" "Öffnungszeiten"`.

Seven usable primary pages were selected by relevance, not result estimates.
Direct retrieval also followed the restaurant website, official guides, and
developer references. The two unusable direct retrievals are recorded above.
This is bounded product-design research, not a new systematic candidate review.

The customer-UX follow-up searched these exact queries, with no date filters and
no pagination: `site.nngroup.com progressive disclosure required optional information`;
`site.baymard.com food delivery product customization add to cart options`;
`site.help.uber.com ubereats customize item add cart options`. Four additional
usable pages (UX08–UX11) were opened and examined at the locators above. Search
snippets, unrelated results, and premium-only material do not support the findings.
