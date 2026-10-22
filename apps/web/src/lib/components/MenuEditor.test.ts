import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, expect, test } from 'vitest';
import type { components } from '$lib/api/generated';
import MenuEditor from './MenuEditor.svelte';

afterEach(cleanup);

test('creates, assigns, renames, reorders, and removes categories without removing dishes', async () => {
  const { container } = render(MenuEditor, { initial: menu(), base: 1 });
  const saved = () => JSON.parse(container.querySelector<HTMLInputElement>('input[name="document"]')!.value) as components['schemas']['FoodMenu'];
  await fireEvent.click(screen.getByRole('button', { name: 'Add category' }));
  await fireEvent.input(screen.getByLabelText('Category name'), { target: { value: 'Main dishes' } });
  const id = saved().categories![0].id;
  await fireEvent.change(screen.getAllByLabelText('Category', { exact: true })[0], { target: { value: id } });
  expect(saved().items[0].category_id).toBe(id);
  await fireEvent.click(screen.getByRole('button', { name: 'Add category' }));
  await fireEvent.click(screen.getByRole('button', { name: /Move.*New category.*up/ }));
  expect(saved().categories![1].id).toBe(id);
  await fireEvent.click(screen.getByRole('button', { name: /Remove category.*Main dishes/ }));
  expect(saved().items).toHaveLength(2);
  expect(saved().items[0].category_id).toBeUndefined();
  expect(saved().categories).toHaveLength(1);
});

function menu(): components['schemas']['FoodMenu'] {
  return {
    currency: 'EUR',
    availability_mode: 'manual',
    items: ['Döner', 'Falafel'].map((name, index) => ({
      id: `item-${index}`, name, available: true,
      variants: [{ id: 'regular', name: 'Regular', price_minor: 700 }],
      option_groups: [],
    })),
  };
}

test('removes the selected item from the submitted draft without changing its source', async () => {
  const initial = menu();
  const { container } = render(MenuEditor, { initial, base: 4 });
  const remove = screen.getByRole('button', { name: /Remove menu item.*Döner/ });
  expect(remove).toHaveAttribute('type', 'button');
  await fireEvent.click(remove);
  expect(screen.getByLabelText('Item name')).toHaveValue('Falafel');
  const document = container.querySelector<HTMLInputElement>('input[name="document"]')!;
  const saved = JSON.parse(document.value) as components['schemas']['FoodMenu'];
  expect(saved.items.map((item) => item.id)).toEqual(['item-1']);
  expect(initial.items).toHaveLength(2);
  expect(container.querySelector('input[name="base"]')).toHaveValue('4');
});

test('can remove the last item and add a replacement before saving', async () => {
  render(MenuEditor, { initial: { ...menu(), items: menu().items.slice(0, 1) }, base: 1 });
  await fireEvent.click(screen.getByRole('button', { name: /Remove menu item.*Döner/ }));
  expect(screen.queryByLabelText('Item name')).not.toBeInTheDocument();
  expect(screen.getByText(/Add at least one item before saving/)).toBeVisible();
  await fireEvent.click(screen.getByRole('button', { name: 'Add menu item' }));
  expect(screen.getByLabelText('Item name')).toHaveValue('New item');
  expect(screen.queryByText(/Add at least one item before saving/)).not.toBeInTheDocument();
});

test('makes optional paid and free extras distinct from size prices and allows both', async () => {
  const { container } = render(MenuEditor, { initial: { ...menu(), items: menu().items.slice(0, 1) }, base: 1 });
  expect(screen.getByRole('heading', { name: 'Sizes and variants' })).toBeVisible();
  expect(screen.getByRole('heading', { name: 'Extras and additions' })).toBeVisible();
  await fireEvent.click(screen.getByRole('button', { name: 'Add extras group' }));
  await fireEvent.input(screen.getByLabelText('Extra name'), { target: { value: 'Extra cheese' } });
  await fireEvent.input(screen.getByLabelText('Extra price in EUR'), { target: { value: '1' } });
  await fireEvent.click(screen.getByRole('button', { name: 'Add extra' }));
  await fireEvent.input(screen.getAllByLabelText('Extra name')[1], { target: { value: 'Garlic' } });
  const saved = JSON.parse(container.querySelector<HTMLInputElement>('input[name="document"]')!.value) as components['schemas']['FoodMenu'];
  const group = saved.items[0].option_groups[0];
  expect(group.min_selected).toBe(0);
  expect(group.max_selected).toBe(2);
  expect(group.options.map(option => [option.name, option.price_minor])).toEqual([['Extra cheese',100],['Garlic',0]]);
  expect(saved.items[0].variants[0].price_minor).toBe(700);
  await fireEvent.click(screen.getByRole('button', { name: /Remove extras group.*Extras/ }));
  expect(screen.getByText(/No extras yet/)).toBeVisible();
});
