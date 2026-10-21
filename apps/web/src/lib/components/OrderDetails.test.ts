import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, expect, test } from 'vitest';
import type { components } from '$lib/api/generated';
import OrderDetails from './OrderDetails.svelte';

afterEach(cleanup);
function order(): components['schemas']['FoodOrder'] {
  const id = '00000000-0000-4000-8000-000000000001';
  return { id, merchant_id: id, store_id: id, contract: 'ocp.food.order.v1', fulfilment: 'pickup', acceptance: 'pending', payment: 'not_started', fulfilment_progress: 'not_started', resolution: 'open', revision: 1, created_at: '2026-10-21T12:00:00Z',
    terms: { id, merchant_id: id, store_id: id, publication_generation: 1, revision: 1, model: { id: 'ocp.food.menu', version: '1.0.0' }, currency: 'EUR', total_minor: 700, created_at: '2026-10-21T12:00:00Z', expires_at: '2026-10-21T12:10:00Z', lines: [{ selection: { item_id: 'doner', variant_id: 'regular', quantity: 1, options: [] }, item_name: 'Döner', variant_name: 'Regular', unit_price_minor: 700, total_minor: 700 }] },
    history: [{ sequence: 1, event: 'placed', created_at: '2026-10-21T12:00:00Z' }] };
}
test('renders agreed terms and pending progress with a durable history', () => {
  render(OrderDetails, { order: order() });
  expect(screen.getByRole('status')).toHaveTextContent('Awaiting restaurant acceptance');
  expect(screen.getByRole('listitem')).toHaveTextContent('Placed');
  expect(screen.getByRole('region', { name: 'Order details' })).toHaveTextContent('7.00');
});
test('shows acceptance without suggesting payment or kitchen progress', () => {
  render(OrderDetails, { order: { ...order(), acceptance: 'accepted', revision: 2 } });
  expect(screen.getByRole('status')).toHaveTextContent('Accepted by the restaurant.');
  expect(screen.getByText('Payment has not started.')).toBeVisible();
  expect(screen.getByText('Preparation has not started.')).toBeVisible();
});
test('gives a closed resolution precedence over prior acceptance', () => {
  render(OrderDetails, { order: { ...order(), acceptance: 'accepted', resolution: 'cancelled', revision: 3 } });
  expect(screen.getByRole('status')).toHaveTextContent('cancelled');
  expect(screen.queryByText('Accepted by the restaurant.', { exact: false })).not.toBeInTheDocument();
});
