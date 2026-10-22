<script lang="ts">
  import type { components } from '$lib/api/generated';
  import { useI18n } from '$lib/i18n/context';
  import { money } from '$lib/money';
  import ShoppingBag from '@lucide/svelte/icons/shopping-bag';
  import Check from '@lucide/svelte/icons/check';
  import X from '@lucide/svelte/icons/x';
  import CreditCard from '@lucide/svelte/icons/credit-card';
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
  import ChefHat from '@lucide/svelte/icons/chef-hat';
  import BellRing from '@lucide/svelte/icons/bell-ring';
  import PackageCheck from '@lucide/svelte/icons/package-check';
  let { order }: { order: components['schemas']['FoodOrder'] } = $props();
  const { t, locale } = useI18n();
  const events = { placed: 'orders-event-placed', accepted: 'orders-event-accepted', rejected: 'orders-event-rejected', cancelled: 'orders-event-cancelled', payment_started: 'payment-event-started', payment_paid: 'payment-event-paid', payment_failed: 'payment-event-failed', payment_abandoned: 'payment-event-abandoned', cancelled_refunded: 'payment-event-refunded', preparation_started: 'fulfilment-event-preparing', ready_for_pickup: 'fulfilment-event-ready', collected: 'fulfilment-event-collected' };
  const icons = { placed: ShoppingBag, accepted: Check, rejected: X, cancelled: X, payment_started: CreditCard, payment_paid: CreditCard, payment_failed: X, payment_abandoned: X, cancelled_refunded: RotateCcw, preparation_started: ChefHat, ready_for_pickup: BellRing, collected: PackageCheck };
  const dateFormat = $derived(new Intl.DateTimeFormat(locale(), { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Europe/Berlin' }));
</script>
<section aria-label={t('receipt-details')}>
  <div class="order-heading"><h2>{t('orders-summary')}</h2><strong class="reference"><bdi dir="ltr">{order.reference ?? order.id}</bdi></strong></div>
  <p class="muted">{t('orders-created')} <time datetime={order.created_at}>{dateFormat.format(new Date(order.created_at))}</time></p>
  <dl class="summary">
    <div><dt>{t('orders-status')}</dt><dd>{t(`admin-state-${order.resolution === 'open' ? order.acceptance : order.resolution}`)}</dd></div>
    <div><dt>{t('admin-payment')}</dt><dd>{t(`admin-payment-${order.payment}`)}</dd></div>
    <div><dt>{t('fulfilment-title')}</dt><dd>{t(`admin-fulfilment-${order.fulfilment_progress}`)}</dd></div>
  </dl>
  <div class="current-progress" role="status">
  {#if order.resolution === 'cancelled'}<p>{t('orders-cancelled')}</p>
  {:else if order.resolution === 'rejected'}<p>{t('orders-rejected')}</p>
  {:else}<p>{t(order.acceptance === 'accepted' ? 'orders-accepted' : 'orders-pending')}</p><p>{t(`fulfilment-status-${order.fulfilment_progress}`)}</p>{/if}
  <p>{t(`payment-status-${order.payment}`)}</p>
  </div>
  <h2>{t('receipt-selection')}</h2>
  <ul class="selection">
  {#each order.terms.lines as line}
    <li><div><strong>{line.selection.quantity} × <bdi dir="auto">{line.item_name}</bdi></strong><span class="muted"><bdi dir="auto">{line.variant_name}</bdi></span>
    {#each line.selected_options ?? [] as option}<small><bdi dir="auto">{option.group_name}</bdi>: <bdi dir="auto">{option.option_name}</bdi></small>{/each}</div><bdi>{money(line.total_minor, locale())}</bdi></li>
  {/each}
  </ul>
  <p class="total"><strong>{t('storefront-total', { amount: money(order.terms.total_minor, locale()) })}</strong></p>
  {#if order.payment_provider === 'development'}<p class="muted">{t('payment-development-help')}</p>{/if}
</section>
  {#if order.history?.length}
  <section aria-label={t('orders-history')}>
    <h2>{t('orders-history')}</h2><p class="muted">{t('orders-timeline-help')}</p>
    <ol class="timeline">{#each order.history as event (event.sequence)}
      {@const Icon = icons[event.event]}
      <li><span class="event-icon"><Icon size={19} aria-hidden="true" /></span><div class="event-body"><strong>{t(events[event.event])}</strong><time datetime={event.created_at}>{dateFormat.format(new Date(event.created_at))}</time></div></li>
    {/each}</ol>
  </section>
  {/if}
<style>
  .order-heading { display: flex; align-items: center; justify-content: space-between; gap: .75rem; flex-wrap: wrap; }
  .order-heading h2 { margin-block: 0; }
  .reference { color: #20586f; background: #edf4f7; border-radius: .3rem; padding: .35rem .6rem; }
  .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: .75rem; margin-block: 1.25rem; }
  .summary div { background: #f1f6f8; border-radius: .3rem; padding: .75rem; }
  dt { font-size: .85rem; color: #526671; } dd { margin: .25rem 0 0; font-weight: 650; }
  .current-progress { border-inline-start: 3px solid #256078; padding-inline-start: .9rem; }
  .current-progress p { margin-block: .35rem; }
  .selection, .timeline { list-style: none; padding: 0; margin: 0; }
  .selection li { display: flex; justify-content: space-between; gap: 1rem; padding-block: .8rem; border-block-end: 1px solid #ccd8de; }
  .selection li div { display: grid; gap: .2rem; }
  .selection li > bdi { white-space: nowrap; }
  .total { text-align: end; }
  .timeline li { position: relative; display: grid; grid-template-columns: 36px minmax(0, 1fr); column-gap: .85rem; padding-block-end: 1.25rem; }
  .timeline li:last-child { padding-block-end: 0; }
  .timeline li:not(:last-child)::before { content: ''; position: absolute; inset-inline-start: 17px; inset-block: 36px 0; border-inline-start: 2px solid #ccd8de; }
  .event-icon { width: 36px; height: 36px; display: flex; align-items: center; justify-content: center; background: #edf4f7; color: #20586f; border: 1px solid #ccd8de; border-radius: 50%; }
  .event-body { display: flex; align-items: baseline; justify-content: space-between; gap: .25rem 1rem; flex-wrap: wrap; padding-block-start: .3rem; }
  .event-body time { color: #526671; font-size: .9rem; }
</style>
