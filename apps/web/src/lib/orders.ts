import type { components } from '$lib/api/generated';
type Order = components['schemas']['FoodOrder'];
export type DevelopmentAction = 'start' | 'succeed' | 'decline' | 'abandon';

export function developmentActions(order: Order, enabled: boolean): DevelopmentAction[] {
  if (!enabled || order.resolution !== 'open' || order.acceptance !== 'accepted' || order.fulfilment_progress !== 'not_started') return [];
  if (order.payment === 'not_started' || order.payment === 'failed') return ['start'];
  return order.payment === 'awaiting_customer' ? ['succeed', 'decline', 'abandon'] : [];
}

export function canCancelOrder(order: Order): boolean {
  return order.resolution === 'open' && order.fulfilment_progress === 'not_started'
    && (order.payment === 'not_started' || order.payment_provider === 'development');
}

export function fulfilmentAction(order: Order): 'start_preparation' | 'mark_ready' | 'record_collection' | undefined {
  if (order.resolution !== 'open' || order.acceptance !== 'accepted' || order.payment !== 'paid' || order.fulfilment !== 'pickup') return;
  switch (order.fulfilment_progress) {
    case 'not_started': return 'start_preparation';
    case 'preparing': return 'mark_ready';
    case 'ready': return 'record_collection';
  }
}
