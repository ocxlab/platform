<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t } = useI18n();
  import OrderDetails from '$lib/components/OrderDetails.svelte';
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import X from '@lucide/svelte/icons/x';
  import CreditCard from '@lucide/svelte/icons/credit-card';
  import OrderLiveUpdates from '$lib/components/OrderLiveUpdates.svelte';
  import { canCancelOrder, developmentActions } from '$lib/orders';
  let { data, form } = $props();
</script>
<h1>{t('receipt-title')}</h1>
<p>{t('receipt-intro')}</p>
{#if form?.error}<Notice text={form.error} error />{/if}
<OrderLiveUpdates url={`${data.receipt}/updates`} refresh={data.receipt} />
<OrderDetails order={data.order} />
{#if developmentActions(data.order, data.developmentPayments).length}
<section aria-label={t('payment-development')}>
  <h2>{t('payment-development')}</h2><p>{t('payment-development-help')}</p>
  {#if data.order.payment === 'awaiting_customer'}
    <a class="checkout-link" data-sveltekit-reload href={`${data.receipt}/checkout`}>{t('payment-resume')}</a>
  {:else}
    <form method="POST" action="?/payment">
      <input type="hidden" name="action" value="start" /><input type="hidden" name="revision" value={data.order.revision} />
      <input type="hidden" name="key" value={`${data.order.id}-${data.order.revision}-pay-start`} />
      <ActionButton icon={CreditCard} label={t('payment-action-start')} />
    </form>
  {/if}
</section>
{/if}
{#if canCancelOrder(data.order)}
  <form method="POST" action="?/cancel"><input type="hidden" name="revision" value={data.order.revision} />
    <input type="hidden" name="key" value={`${data.order.id}-${data.order.revision}-cancel`} />
    <ActionButton icon={X} label={t('orders-cancel')} danger />
  </form>
{/if}
<p>{t('receipt-cookie-help')}</p>
<a href={data.storefront}>{t('receipt-back')}</a>
