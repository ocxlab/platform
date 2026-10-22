import { expect, test } from 'vitest';
import { basketPrice, basketSignature, menuName, validBasketLine, type FoodMenu } from './food-menu';
const menu: FoodMenu = { currency: 'EUR', availability_mode: 'manual', items: [{ id: 'pizza', name: 'Pizza', translations: { ar: 'بيتزا' }, available: true,
  variants: [{ id: 'large', name: 'Large', price_minor: 1000 }], option_groups: [{ id: 'extras', name: 'Extras', min_selected: 0, max_selected: 2,
    options: [{ id: 'cheese', name: 'Cheese', price_minor: 100 }, { id: 'garlic', name: 'Garlic', price_minor: 0 }] }] }] };
test('localizes labels with a stable source-name fallback', () => {
  expect(menuName(menu.items[0], 'ar')).toBe('بيتزا');
  expect(menuName(menu.items[0], 'de')).toBe('Pizza');
});
test('estimates variants and free/paid extras for every quantity', () => {
  expect(basketPrice(menu, { item_id: 'pizza', variant_id: 'large', quantity: 2, options: [{ group_id: 'extras', option_id: 'cheese' }, { group_id: 'extras', option_id: 'garlic' }] })).toBe(2200);
});
test('cart signature ignores checkbox order but distinguishes quantities and notes', () => {
  const line = { item_id: 'pizza', variant_id: 'large', quantity: 1, options: [{ group_id: 'extras', option_id: 'cheese' }, { group_id: 'extras', option_id: 'garlic' }] };
  expect(basketSignature([line], '')).toBe(basketSignature([{ ...line, options: [...line.options].reverse() }], ''));
  expect(basketSignature([line], '')).not.toBe(basketSignature([{ ...line, quantity: 2 }], ''));
  expect(basketSignature([line], '')).not.toBe(basketSignature([{ ...line, note: 'No onions' }], ''));
  expect(basketSignature([line], '')).not.toBe(basketSignature([line], 'Bring a bag'));
});
test('validates bounded stored lines without trusting browser objects', () => {
  const line = { item_id: 'pizza', variant_id: 'large', quantity: 1, options: [] };
  expect(validBasketLine(line)).toBe(true);
  for (const bad of [null, {}, { ...line, quantity: 100 }, { ...line, quantity: 1.5 }, { ...line, item_id: '../x' }, { ...line, note: 'x'.repeat(501) }, { ...line, options: [null] }]) expect(validBasketLine(bad)).toBe(false);
});
