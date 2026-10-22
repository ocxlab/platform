import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, expect, test } from 'vitest';
import MenuEditor from './MenuEditor.svelte';
import type { FoodMenu } from '$lib/food-menu';
afterEach(cleanup);

test('edits size overrides and an explicit reusable meal without duplicating catalog items', async () => {
  const menu: FoodMenu = { currency: 'EUR', availability_mode: 'manual', items: [
    { id: 'pizza', name: 'Pizza', available: true, variants: [{ id: 'large', name: 'Large', price_minor: 1000 }], option_groups: [{ id: 'extras', name: 'Extras', min_selected: 0, max_selected: 1, options: [{ id: 'cheese', name: 'Cheese', price_minor: 100 }] }] },
    { id: 'salad', name: 'Salad', available: true, variants: [{ id: 'regular', name: 'Regular', price_minor: 650 }], option_groups: [] }
  ] };
  const { container } = render(MenuEditor,{ initial: menu, base: 1 });
  const saved = () => JSON.parse(container.querySelector<HTMLInputElement>('input[name="document"]')!.value) as FoodMenu;
  await fireEvent.input(screen.getByLabelText('Extra price · Cheese · Large'), { target: { value: '1.5' } });
  expect(saved().items[0].option_groups[0].options[0].variant_prices).toEqual({ large: 150 });
  await fireEvent.input(screen.getByLabelText('Extra price · Cheese · Large'), { target: { value: '0' } });
  expect(saved().items[0].option_groups[0].options[0].variant_prices).toEqual({ large: 0 });
  await fireEvent.input(screen.getByLabelText('Extra price · Cheese · Large'), { target: { value: '' } });
  expect(saved().items[0].option_groups[0].options[0].variant_prices).toBeUndefined();
  await fireEvent.click(screen.getByRole('button',{ name: 'Add meal deal' }));
  await fireEvent.input(screen.getByLabelText('Deal name'),{ target: { value: 'Healthy Pizza' } });
  await fireEvent.click(screen.getByLabelText('Pizza', { exact: true }));
  await fireEvent.click(screen.getByLabelText('Salad · Regular'));
  expect(saved().meal_deals?.[0]).toMatchObject({ name: 'Healthy Pizza', main_item_ids: ['pizza'], surcharge_minor: 100, addon_choices: [{ item_id: 'salad', variant_id: 'regular' }] });
  expect(saved().items).toHaveLength(2);
  await fireEvent.click(screen.getByRole('button',{ name: /Remove menu item.*Salad/ }));
  expect(saved().meal_deals).toEqual([]);
});
