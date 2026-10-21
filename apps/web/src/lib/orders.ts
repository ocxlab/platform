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
