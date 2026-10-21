<script lang="ts">
  import type { components } from '$lib/api/generated';
  import { useI18n } from '$lib/i18n/context';
  import { money } from '$lib/money';
  let { order }: { order: components['schemas']['FoodOrder'] } = $props();
  const { t, locale } = useI18n();
  const events = { placed: 'orders-event-placed', accepted: 'orders-event-accepted', rejected: 'orders-event-rejected', cancelled: 'orders-event-cancelled', payment_started: 'payment-event-started', payment_paid: 'payment-event-paid', payment_failed: 'payment-event-failed', payment_abandoned: 'payment-event-abandoned', cancelled_refunded: 'payment-event-refunded' };
</script>
<section aria-label={t('receipt-details')}>
  <h2>{t('receipt-selection')}</h2>
  {#each order.terms.lines as line}
    <p>{line.selection.quantity} × <bdi dir="auto">{line.item_name}</bdi> (<bdi dir="auto">{line.variant_name}</bdi>) · <bdi>{money(line.total_minor, locale())}</bdi>
    {#each line.selected_options ?? [] as option}<br /><small><bdi dir="auto">{option.group_name}</bdi>: <bdi dir="auto">{option.option_name}</bdi></small>{/each}</p>
  {/each}
  <p><strong>{t('storefront-total', { amount: money(order.terms.total_minor, locale()) })}</strong></p>
  {#if order.resolution === 'cancelled'}<p role="status">{t('orders-cancelled')}</p>
  {:else if order.resolution === 'rejected'}<p role="status">{t('orders-rejected')}</p>
  {:else if order.acceptance === 'accepted'}<p role="status">{t('orders-accepted')}</p>
  {:else}<p role="status">{t('orders-pending')}</p>{/if}
  <p>{t(`payment-status-${order.payment}`)}</p>
  {#if order.payment_provider === 'development'}<p class="muted">{t('payment-development-help')}</p>{/if}
  <p>{t('payment-preparation')}</p>
  <p class="muted">{t('receipt-reference')} <bdi dir="ltr">{order.reference ?? order.id}</bdi></p>
  {#if order.history?.length}
    <h3>{t('orders-history')}</h3>
    <ol>{#each order.history as event}<li>{t(events[event.event])} · <time datetime={event.created_at}>{new Date(event.created_at).toLocaleString(locale())}</time></li>{/each}</ol>
  {/if}
</section>
