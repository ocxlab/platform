import type { components } from '$lib/api/generated';

// HTML controls submit their actual values even before hydration. Prices,
// availability, and option constraints remain authoritative in the commerce API.
export function foodSelection(form: FormData): components['schemas']['FoodSelectionRequest'] | undefined {
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
  return { publication_generation: generation, lines: [{ item_id: item, variant_id: variant, quantity, options }] };
}
