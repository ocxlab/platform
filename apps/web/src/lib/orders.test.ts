import { expect, test } from 'vitest';
import type { components } from '$lib/api/generated';
import { canCancelOrder, developmentActions, fulfilmentAction } from './orders';
type Order = components['schemas']['FoodOrder'];
const order = (changes: Partial<Order> = {}) => ({ resolution: 'open', acceptance: 'accepted', payment: 'not_started', fulfilment_progress: 'not_started', ...changes }) as Order;

test('development checkout requires enablement and accepted, open, unprepared order', () => {
  expect(developmentActions(order(), false)).toEqual([]);
  expect(developmentActions(order(), true)).toEqual(['start']);
  for (const change of [{ resolution: 'cancelled' }, { resolution: 'rejected' }, { acceptance: 'pending' }, { acceptance: 'rejected' }, { fulfilment_progress: 'preparing' }]) {
    expect(developmentActions(order(change as Partial<Order>), true)).toEqual([]);
  }
});

test('pickup actions require accepted paid orders and advance without skipping work', () => {
  const paid = order({ payment: 'paid', fulfilment: 'pickup' });
  expect(fulfilmentAction(paid)).toBe('start_preparation');
  expect(fulfilmentAction({ ...paid, fulfilment_progress: 'preparing' })).toBe('mark_ready');
  expect(fulfilmentAction({ ...paid, fulfilment_progress: 'ready' })).toBe('record_collection');
  expect(fulfilmentAction({ ...paid, fulfilment_progress: 'collected' })).toBeUndefined();
  for (const change of [{payment:'not_started'},{payment:'awaiting_customer'},{payment:'failed'},{payment:'refunded'},{acceptance:'pending'},{resolution:'cancelled'},{fulfilment:'delivery'}]) {
    expect(fulfilmentAction({ ...paid, ...change } as Order)).toBeUndefined();
  }
});

test('checkout actions track payment outcome', () => {
  expect(developmentActions(order({ payment: 'failed' }), true)).toEqual(['start']);
  expect(developmentActions(order({ payment: 'awaiting_customer' }), true)).toEqual(['succeed','decline','abandon']);
  expect(developmentActions(order({ payment: 'paid' }), true)).toEqual([]);
  expect(developmentActions(order({ payment: 'refunded' }), true)).toEqual([]);
});

test('cancellation never suggests unsupported real refunds or prepared/closed cancellation', () => {
  expect(canCancelOrder(order())).toBe(true);
  expect(canCancelOrder(order({ payment: 'paid' }))).toBe(false);
  expect(canCancelOrder(order({ payment: 'failed' }))).toBe(false);
  expect(canCancelOrder(order({ payment: 'awaiting_customer' }))).toBe(false);
  for (const payment of ['awaiting_customer','failed','paid'] as const) {
    expect(canCancelOrder(order({ payment, payment_provider:'development' }))).toBe(true);
  }
  expect(canCancelOrder(order({ resolution:'cancelled' }))).toBe(false);
  expect(canCancelOrder(order({ resolution:'rejected', payment_provider:'development' }))).toBe(false);
  expect(canCancelOrder(order({ fulfilment_progress:'preparing' } as unknown as Partial<Order>))).toBe(false);
});
