import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, expect, test } from 'vitest';
import type { components } from '$lib/api/generated';
import MenuEditor from './MenuEditor.svelte';

afterEach(cleanup);

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
