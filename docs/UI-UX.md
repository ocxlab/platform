# Interface conventions

The interface should make routine work discoverable without copying identifiers
or learning protocol vocabulary. These conventions guide incremental UI work;
they are not a claim of completed accessibility certification or user research.

## Browse before looking up identifiers

Use named records, searchable tables, and row actions for management screens.
Keep UUIDs as API identity, not a required navigation step. The provider admin
offers Stores and Orders areas using ordinary navigation links, not a simulated
ARIA tab widget. Its tables use aligned logical-start headers and cells, readable
references, status text, and horizontally scrollable containers on small screens.

Search, status filters, and page size are explicit controls. Changing filters
starts a new search. Server-side keyset pages bound responses without pretending
to know a stable total; these are live lists, not snapshots. Next and First page
links always remain available when applicable; backward links retain at most 20
positions. Refresh reads the server again. Tables show summary data; details are
requested separately through authorized APIs. An empty result and a failed read
are different states.

## Planned administration workspace and filter toolbar

Provider administration should be a dedicated full-page workspace within the
existing SvelteKit app, with its own layout/navigation and separately authorized
APIs. It does not require a new frontend deployment. Use the available width for
management tables while retaining a usable responsive layout.

For the upcoming admin UX pass, evaluate a search-submit icon inside the search
field, an icon-only clear-filters button, and aligned search/filter/page-size
controls on wide screens. Give icon buttons accessible names and supplementary
tooltips; preserve Enter-to-search and keyboard focus. Wrap controls predictably
on narrow screens and mirror their layout using logical properties for Arabic.
Show applied filters clearly, using removable chips where they improve clarity.
Distinguish text being edited from filters actually applied to the result list.
A compact filter panel can disclose less frequent filters without hiding active
ones. These are requested design directions, not a claim that one search pattern
is mandatory or that these changes are already implemented.

## Explain actions where people need them

Use a visible verb and label for less obvious actions. Reserve icon-only buttons
for familiar compact actions and retain accessible names. Tooltips supplement
those buttons with readable text on hover or keyboard focus and dismiss on Escape.
Do not repeat an already visible button label in a tooltip. Essential guidance
must not depend on hover, focus, or a tooltip.

Keep short field guidance visible. Use an expandable details panel for longer,
optional explanations. For menus, sizes/variants carry their full price; extras
add a surcharge, including zero. Separate those sections and label the creation
actions “Add variant,” “Add extras group,” and “Add extra.” Selection limits are
available under Selection rules. Adding an option expands an all-options-allowed
group's maximum; it does not override a deliberately narrower maximum or minimum.
Customer-facing prices show zero-priced extras explicitly.

## Preserve access and language boundaries

The browser uses the same versioned application APIs as other clients, never a
direct database connection. Administration remains a separate credential audience;
order inspection does not confer merchant decision or payment authority.
Consequential visibility changes still require a reason and expected generation.

Provide English, German, and Arabic labels and help together. Use logical CSS
properties and text alignment, isolate merchant-entered names, and keep references
left-to-right inside RTL pages. Verify keyboard, narrow-screen, and no-JavaScript
navigation where the feature supports it; a native form does not make the dynamic
menu editor JavaScript-free.

## Visual menu browsing

The customer menu opens with category sections and image cards. List view uses
the same dishes with smaller photos; a grouped dropdown remains available for
compact selection. Search narrows every view, and category links jump directly
to their section. Display the lowest size price as “From” when variants exist;
the server still calculates the actual selection including extras. Sold-out
dishes remain visible and cannot be chosen.

Choosing a dish reveals its customization form and moves keyboard focus to its
heading. Sizes, extras, and quantity are disclosed at that point, not repeated
on every card. The price check and explicit placement remain separate. This is
still the existing single-dish selection flow, not a new multi-dish cart.
Real item links and category anchors work without JavaScript; view switching
and live menu search require JavaScript and are disabled until it is ready.
Merchant category changes follow the same draft/save/publish process as dishes.

This pattern is informed by [Uber Eats' menu structure guidance](https://merchants.ubereats.com/ar/en/resources/learning-center/menu-and-photos/)
(categories, items, and modifier groups) and [Deliveroo's category management](https://help.deliveroo.com/en/articles/6060280-how-to-add-edit-and-remove-menu-categories-in-menu-manger)
(add, rename, reorder, and publish categories), examined 22 October 2026. Neither
source establishes cards as a universal best layout: our default and view choices
are a design decision to evaluate with the illustrated 48-item demo and merchants.

## Other pattern references

The [WAI tooltip pattern](https://www.w3.org/WAI/ARIA/apg/patterns/tooltip/)
describes focus/hover activation, Escape dismissal, and supplementary non-focusable
content; WAI explicitly marks that pattern as work in progress. The
[GOV.UK text-input guidance](https://design-system.service.gov.uk/components/text-input/)
and [details component](https://design-system.service.gov.uk/components/details/)
distinguish short field hints from longer optional explanations. These inform
interaction choices, not adoption of either project's entire visual design.
