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

## Pattern references

The [WAI tooltip pattern](https://www.w3.org/WAI/ARIA/apg/patterns/tooltip/)
describes focus/hover activation, Escape dismissal, and supplementary non-focusable
content; WAI explicitly marks that pattern as work in progress. The
[GOV.UK text-input guidance](https://design-system.service.gov.uk/components/text-input/)
and [details component](https://design-system.service.gov.uk/components/details/)
distinguish short field hints from longer optional explanations. These inform
interaction choices, not adoption of either project's entire visual design.
