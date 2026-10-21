import { describe, expect, it } from 'vitest';
import { foodSelection } from './food-selection';

function form() {
  const value = new FormData();
  for (const [key, item] of Object.entries({ item_id: 'doner', variant_id: 'regular', publication_generation: '3', quantity: '2' })) value.set(key, item);
  return value;
}
describe('native food selection form', () => {
  it('projects the submitted controls without accepting a price from the browser', () => {
    const value = form(); value.append('option', 'sauce:garlic'); value.set('total_minor', '1');
    expect(foodSelection(value)).toEqual({ publication_generation: 3, lines: [{ item_id: 'doner', variant_id: 'regular', quantity: 2, options: [{ group_id: 'sauce', option_id: 'garlic' }] }] });
  });
  it('allows no extras and quantity bounds', () => {
    for (const quantity of ['1', '99']) { const value = form(); value.set('quantity', quantity); expect(foodSelection(value)?.lines[0].quantity).toBe(Number(quantity)); }
    expect(foodSelection(form())?.lines[0].options).toEqual([]);
  });
  it('rejects missing, malformed, fractional, and out of range fields', () => {
    for (const [field, invalid] of [['item_id', ''], ['variant_id', 'bad:id'], ['publication_generation', '0'], ['publication_generation', '9007199254740992'], ['quantity', '0'], ['quantity', '100'], ['quantity', '1.5'], ['quantity', 'no']]) {
      const value = form(); value.set(field, invalid); expect(foodSelection(value)).toBeUndefined();
    }
    const missing = form(); missing.delete('quantity'); expect(foodSelection(missing)).toBeUndefined();
  });
  it('rejects malformed options and oversized option lists', () => {
    for (const option of ['sauce', 'sauce:garlic:extra', ':garlic', 'sauce:bad.id']) { const value = form(); value.append('option', option); expect(foodSelection(value)).toBeUndefined(); }
    const value = form(); for (let index = 0; index < 601; index++) value.append('option', 'sauce:garlic'); expect(foodSelection(value)).toBeUndefined();
  });
});
