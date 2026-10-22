import { describe, expect, it, test } from 'vitest';
import { foodSelection } from './food-selection';
function form() {
  const value = new FormData();
  for (const [key, item] of Object.entries({ item_id: 'doner', variant_id: 'regular', publication_generation: '3', quantity: '2' })) value.set(key, item);
  return value;
}
describe('native food selection form', () => {
  it('projects a multi-item basket and bounded notes, never browser prices', () => {
    const value = new FormData();
    value.set('basket', JSON.stringify({ publication_generation: 2, locale: 'ar', note: 'Please call', total_minor: 1,
      lines: [{ item_id: 'pizza', variant_id: 'large', quantity: 2, options: [], note: 'Cut in slices', price_minor: 1 }, { item_id: 'water', variant_id: 'one', quantity: 1, options: [] }] }));
    expect(foodSelection(value)).toEqual({ publication_generation: 2, locale: 'ar', note: 'Please call', lines: [
      { item_id: 'pizza', variant_id: 'large', quantity: 2, options: [], note: 'Cut in slices' }, { item_id: 'water', variant_id: 'one', quantity: 1, options: [] }] });
    for (const invalid of [[], Array.from({ length: 21 }, () => ({ item_id: 'x', variant_id: 'y', quantity: 1, options: [] }))]) {
      value.set('basket', JSON.stringify({ publication_generation: 1, locale: 'en', lines: invalid })); expect(foodSelection(value)).toBeUndefined();
    }
    value.set('basket', 'null'); expect(foodSelection(value)).toBeUndefined();
    value.set('basket', '{'); expect(foodSelection(value)).toBeUndefined();
  });
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

test('forwards only explicit meal references, never a browser-supplied surcharge', () => {
  const form = new FormData();
  const line = { item_id: 'pizza', variant_id: 'large', quantity: 2, options: [], meal_deal: { deal_id: 'healthy', addon_item_id: 'salad', addon_variant_id: 'regular', surcharge_minor: -999 } };
  form.set('basket',JSON.stringify({ publication_generation: 1, locale: 'en', lines: [line] }));
  expect(foodSelection(form)?.lines[0].meal_deal).toEqual({ deal_id: 'healthy', addon_item_id: 'salad', addon_variant_id: 'regular' });
});
test('native forms use the same explicit meal references', () => {
  const form = new FormData();
  for (const [key,value] of Object.entries({ publication_generation: '1', item_id: 'pizza', variant_id: 'large', quantity: '1', meal_deal: 'healthy:salad:regular' })) form.set(key,value);
  expect(foodSelection(form)?.lines[0].meal_deal?.addon_item_id).toBe('salad');
  form.set('meal_deal','healthy:../salad:regular');
  expect(foodSelection(form)).toBeUndefined();
});

test('basket transport preserves bounded choices and notes while rejecting malformed input', () => {
  const line = { item_id: 'pizza', variant_id: 'large', quantity: 99, options: [{ group_id: 'extras', option_id: 'cheese' }], note: '😀'.repeat(500) };
  const value = { publication_generation: 1, locale: 'ar', note: '😀'.repeat(500), lines: [line] };
  const parse = (value: unknown) => { const form = new FormData(); form.set('basket',JSON.stringify(value)); return foodSelection(form); };
  expect(parse(value)).toEqual(value);
  expect(parse({ ...value, lines: Array.from({ length:20 }, () => line) })?.lines).toHaveLength(20);
  for (const invalid of [null, [], {}, { ...value, publication_generation: 0 }, { ...value, publication_generation: 1.5 },
    { ...value, publication_generation: Number.MAX_SAFE_INTEGER + 1 }, { ...value, lines: null }, { ...value, lines: [] },
    { ...value, lines: Array.from({ length:21 }, () => line) }, { ...value, lines: [null] }, { ...value, note: 4 },
    { ...value, note: 'x'.repeat(501) }, { ...value, locale: 'fr' }]) expect(parse(invalid)).toBeUndefined();
  const form = new FormData(); form.set('basket','{'); expect(foodSelection(form)).toBeUndefined();
  form.set('basket',' '.repeat(100001)); expect(foodSelection(form)).toBeUndefined();
  // At the maximum transport length, the body is still accepted.
  const raw = JSON.stringify(value); form.set('basket',raw.padEnd(100000)); expect(foodSelection(form)).toEqual(value);
});

test('native form boundaries reject invalid identifiers, quantities, options and meal handles', () => {
  const fields = { publication_generation: '1', item_id: 'pizza', variant_id: 'large', quantity: '1' };
  const native = (patch: Record<string,string> = {}, options: (string | Blob)[] = [], meal?: string | Blob) => {
    const form = new FormData();
    for (const [key,value] of Object.entries({ ...fields,...patch })) form.set(key,value);
    for (const value of options) form.append('option',value);
    if (meal !== undefined) form.set('meal_deal',meal);
    return foodSelection(form);
  };
  expect(native()).toEqual({ publication_generation: 1, lines: [{ item_id: 'pizza', variant_id: 'large', quantity: 1, options: [] }] });
  expect(native({ quantity:'99' }, ['extras:cheese'])).toEqual({ publication_generation: 1, lines: [{ item_id: 'pizza', variant_id: 'large', quantity: 99, options: [{ group_id:'extras', option_id:'cheese' }] }] });
  expect(native({},Array(600).fill('extras:cheese'))?.lines[0].options).toHaveLength(600);
  for (const patch of [{ item_id:'../x' }, { item_id:'' }, { variant_id:'../x' }, { variant_id:'' },
    { publication_generation:'0' }, { publication_generation:'1.2' }, { publication_generation:'9007199254740992' },
    { quantity:'0' }, { quantity:'100' }, { quantity:'1.5' }, { quantity:'NaN' }] as Record<string,string>[]) expect(native(patch)).toBeUndefined();
  for (const options of [['a:b:c'], ['../a:b'], ['a:../b'], Array(601).fill('a:b'), [new Blob(['x'])]]) expect(native({},options)).toBeUndefined();
  expect(native({},[], '')).toEqual(native());
  for (const meal of ['a:b', 'a:b:c:d', '../a:b:c', 'a:../b:c', 'a:b:../c', new Blob(['x'])]) expect(native({},[],meal)).toBeUndefined();
  expect(native({ item_id:'x'.repeat(64), variant_id:'Z_0-' })).toBeDefined();
  expect(native({ item_id:'x'.repeat(65) })).toBeUndefined();
  expect(native({ variant_id:'x'.repeat(65) })).toBeUndefined();
  const missing = new FormData(); expect(foodSelection(missing)).toBeUndefined();
  for (const field of ['item_id','variant_id']) {
    const form = new FormData(); for (const [k,v] of Object.entries(fields)) form.set(k,v);
    form.set(field,new Blob(['x'])); expect(foodSelection(form)).toBeUndefined();
  }
});
