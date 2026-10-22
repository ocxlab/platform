# Food consolidation review

23 October 2026. Two existing technical agents reviewed the same uncommitted
implementation in their engineering and protocol roles. Both reviews were
read-only/static, not independently executed tests or evidence of external
interoperability. Parent-run commands are recorded in `../STATUS.md`.

## Protected decisions

The newer Food implementation is the sole pre-release V1 model. Remove the old
version split and scheduling, retain size-dependent extras, and add explicit
main-plus-addon meal deals. One addon is included per main unit; surcharge-only
pricing does not also charge its standalone price. Menu references, availability,
tenant scope, and authoritative purchase terms remain enforced by the backend.

## Engineering disposition

The engineering reviewer found no server-side correctness, security or retained-
terms blocker. It checked catalog references, selected-size prices, addon rules,
per-unit quantity, and placement revalidation without repricing.

- Stale V2/scheduling documentation and dead `PriceExplanation` fields contradicted
  the agreed model. Removed them from the active contract/code and replaced the
  obsolete pricing note with `../FOOD-MENU.md`. Existing checkpoint reviews remain
  historical; the acceptance summary marks its old mutation report accordingly.
- No-JavaScript option labels displayed the base extra price even for sizes with
  overrides. Those controls now explain that the price depends on size; the
  returned authoritative quote displays actual extra prices and the included meal.

## Protocol disposition

The protocol reviewer found the bounded meal semantics sound: line-level explicit
selection, quantity-derived included items, surcharge-only calculation, required-
modifier exclusion, availability checks, localization and retained labels.

- Its initial request for historical scheduled-term decoding was based on a
  mistaken reading that the founder required it. The founder requested the
  opposite: one pre-release model without legacy-version support. The checked
  development database contains no V2 quotes/orders or menu revisions. Unused
  scheduling fields were removed instead of inventing a migration requirement.
- Added HTTP regression coverage for an issued quote whose deal is removed from
  the next publication, alongside price-only changes, unavailable included items,
  explicit retired-model rejection and successful replay. Required-addon modifiers
  are covered by schema/semantic and direct-pricer unit tests.

## UX follow-up

The founder requested discovery before upgrade, optional contextual offers, and
consistent styled tooltips. Dish hints disclose meal availability and additional
price; basket actions reopen the existing configuration without an automatic
upgrade. Header, language and menu controls share the action-button tooltip style.
The evidence-informed placement rationale is recorded in `../UI-UX.md`; it is not
a claim of user-tested conversion or industry-wide uniformity.

The engineering agent also reviewed this follow-up statically. It found no
concrete runtime or accessibility blocker in optional upgrades or tooltip
labeling, focus/hover activation, Escape dismissal, cleanup and dialog placement.
That review is not an accessibility certification or a separately executed test.
