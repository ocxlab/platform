<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { createQuery, QueryClient } from '@tanstack/svelte-query';
  import { receiptQuery, type ReceiptOrder } from '$lib/receipt-query';
  import { useI18n } from '$lib/i18n/context';
  import OrderDetails from './OrderDetails.svelte';
  import ActionButton from './ActionButton.svelte';
  import OrderLiveUpdates from './OrderLiveUpdates.svelte';
  import X from '@lucide/svelte/icons/x';
  import CreditCard from '@lucide/svelte/icons/credit-card';
  import { canCancelOrder, developmentActions } from '$lib/orders';
  let { initialOrder, receipt, storefront, developmentPayments }: {
    initialOrder: ReceiptOrder; receipt: string; storefront: string; developmentPayments: boolean;
  } = $props();
  const { t } = useI18n();
  // A component-local client: no cross-request/user cache or persisted secrets.
  const client = new QueryClient();
  const options = $derived(receiptQuery(receipt, initialOrder));
  const query = createQuery(() => options, () => client);
  const order = $derived(query.data);
  onMount(() => { client.mount(); return () => client.unmount(); });
  onDestroy(() => client.clear());

  async function refreshOrder(signal: AbortSignal) {
    signal.throwIfAborted();
    const cancel = () => { void client.cancelQueries({ queryKey: options.queryKey, exact: true }); };
    signal.addEventListener('abort', cancel, { once: true });
    try { await query.refetch({ throwOnError: true, cancelRefetch: false }); }
    finally { signal.removeEventListener('abort', cancel); }
  }
</script>
<OrderLiveUpdates url={`${receipt}/updates`} refresh={receipt} read={refreshOrder} />
<OrderDetails order={order} />
{#if developmentActions(order, developmentPayments).length}
<section aria-label={t('payment-development')}>
  <h2>{t('payment-development')}</h2><p>{t('payment-development-help')}</p>
  {#if order.payment === 'awaiting_customer'}
    <a class="checkout-link" data-sveltekit-reload href={`${receipt}/checkout`}>{t('payment-resume')}</a>
  {:else}
    <form method="POST" action="?/payment">
      <input type="hidden" name="action" value="start" /><input type="hidden" name="revision" value={order.revision} />
      <input type="hidden" name="key" value={`${order.id}-${order.revision}-pay-start`} />
      <ActionButton icon={CreditCard} label={t('payment-action-start')} />
    </form>
  {/if}
</section>
{/if}
{#if canCancelOrder(order)}
  <form method="POST" action="?/cancel"><input type="hidden" name="revision" value={order.revision} />
    <input type="hidden" name="key" value={`${order.id}-${order.revision}-cancel`} />
    <ActionButton icon={X} label={t('orders-cancel')} danger />
  </form>
{/if}
<p>{t('receipt-cookie-help')}</p>
<a href={storefront}>{t('receipt-back')}</a>
