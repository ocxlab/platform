import type { components } from '$lib/api/generated';
import { validBasketLine, type BasketLine } from '$lib/food-menu';

// HTML controls submit their actual values even before hydration. Prices,
// availability, and option constraints remain authoritative in the commerce API.
export function foodSelection(form: FormData): components['schemas']['FoodSelectionRequest'] | undefined {
  const basket = form.get('basket');
  if (typeof basket === 'string') {
    if (basket.length > 100000) return undefined;
    try {
      const value = JSON.parse(basket);
      if (!Number.isSafeInteger(value.publication_generation) || value.publication_generation < 1
        || !Array.isArray(value.lines) || value.lines.length < 1 || value.lines.length > 20 || !value.lines.every(validBasketLine)
        || (value.note !== undefined && (typeof value.note !== 'string' || [...value.note].length > 500))
        || !['en', 'de', 'ar'].includes(value.locale)) return undefined;
      // Enumerate inputs: never forward caller-supplied totals or extra fields.
      return { publication_generation: value.publication_generation, locale: value.locale, note: value.note,
        lines: value.lines.map((line: BasketLine) => ({ item_id: line.item_id, variant_id: line.variant_id,
          quantity: line.quantity, options: line.options.map(o => ({ group_id: o.group_id, option_id: o.option_id })),
          ...(line.meal_deal ? { meal_deal: { deal_id: line.meal_deal.deal_id, addon_item_id: line.meal_deal.addon_item_id, addon_variant_id: line.meal_deal.addon_variant_id } } : {}), ...(line.note ? { note: line.note } : {}) })) };
    } catch { return undefined; }
  }
  const identifier = /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/;
  const item = form.get('item_id');
  const variant = form.get('variant_id');
  const generation = Number(form.get('publication_generation'));
  const quantity = Number(form.get('quantity'));
  if (typeof item !== 'string' || !identifier.test(item) || typeof variant !== 'string' || !identifier.test(variant)
    || !Number.isSafeInteger(generation) || generation < 1 || !Number.isInteger(quantity) || quantity < 1 || quantity > 99) return undefined;
  const values = form.getAll('option');
  if (values.length > 600) return undefined;
  const options: { group_id: string; option_id: string }[] = [];
  for (const value of values) {
    if (typeof value !== 'string') return undefined;
    const parts = value.split(':');
    if (parts.length !== 2 || !identifier.test(parts[0]) || !identifier.test(parts[1])) return undefined;
    options.push({ group_id: parts[0], option_id: parts[1] });
  }
  const meal = form.get('meal_deal');
  let selectedMeal: BasketLine['meal_deal'];
  if (meal !== null && meal !== '') {
    if (typeof meal !== 'string') return undefined;
    const parts = meal.split(':');
    if (parts.length !== 3 || !parts.every(value => identifier.test(value))) return undefined;
    selectedMeal = { deal_id: parts[0], addon_item_id: parts[1], addon_variant_id: parts[2] };
  }
  return { publication_generation: generation, lines: [{ item_id: item, variant_id: variant, quantity, options, ...(selectedMeal ? { meal_deal: selectedMeal } : {}) }] };
}
