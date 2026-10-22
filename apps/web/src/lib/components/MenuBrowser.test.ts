import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, expect, test, vi } from 'vitest';
import type { components } from '$lib/api/generated';
import MenuBrowser from './MenuBrowser.svelte';

afterEach(cleanup);
function menu(): components['schemas']['FoodMenu'] {
  return { currency: 'EUR', availability_mode: 'manual', categories: [{ id: 'pizza', name: 'Pizzas' }], items: [
    { id: 'pizza', category_id: 'pizza', name: 'Margherita', available: true, image_id: 'test-photo', variants: [{ id: 'small', name: 'Small', price_minor: 800 }, { id: 'large', name: 'Large', price_minor: 1000 }], option_groups: [] },
    { id: 'bread', name: 'Bread', available: false, variants: [{ id: 'one', name: 'One', price_minor: 200 }], option_groups: [] },
  ] };
}
test('shows categorized image cards and real selection links before customization', async () => {
  const onchoose = vi.fn();
  render(MenuBrowser, { menu: menu(), merchant: 'merchant', store: 'store', selected: '', onchoose });
  expect(screen.getByRole('heading', { name: 'Pizzas' })).toBeVisible();
  expect(screen.getByRole('heading', { name: 'Other dishes' })).toBeVisible();
  expect(screen.getByRole('img', { name: 'Margherita' })).toHaveAttribute('src', '/media/public/merchant/store/test-photo');
  const dish = screen.getByRole('link', { name: /Choose.*Margherita/ });
  expect(dish).toHaveAttribute('href', '?item=pizza#configure-item');
  expect(screen.getByText(/From.*8.00/)).toBeVisible();
  await fireEvent.click(dish);
  expect(onchoose).toHaveBeenCalledWith('pizza');
  await fireEvent.click(screen.getByRole('link', { name: /Choose.*Bread/ }));
  expect(onchoose).toHaveBeenCalledTimes(1);
});
test('switches views without losing categories and filters all views', async () => {
  const onchoose = vi.fn();
  const { container } = render(MenuBrowser, { menu: menu(), merchant: 'merchant', store: 'store', selected: '', onchoose });
  await fireEvent.click(screen.getByRole('button', { name: 'List' }));
  expect(container.querySelectorAll('.menu-list .menu-card')).toHaveLength(2);
  await fireEvent.input(screen.getByLabelText('Search the menu'), { target: { value: '  MARGHERITA  ' } });
  expect(screen.queryByRole('link', { name: /Choose.*Bread/ })).not.toBeInTheDocument();
  await fireEvent.click(screen.getByRole('button', { name: 'Dropdown' }));
  await fireEvent.change(screen.getByRole('combobox'), { target: { value: 'pizza' } });
  expect(onchoose).toHaveBeenCalledWith('pizza');
  await fireEvent.input(screen.getByLabelText('Search the menu'), { target: { value: 'no such dish' } });
  expect(screen.getByRole('status')).toHaveTextContent('No dishes match your search.');
});
