<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t } = useI18n();
  import OrderDetails from '$lib/components/OrderDetails.svelte';
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import X from '@lucide/svelte/icons/x';
  import CreditCard from '@lucide/svelte/icons/credit-card';
  import { canCancelOrder, developmentActions } from '$lib/orders';
  let { data, form } = $props();
</script>
<h1>{t('receipt-title')}</h1>
<p>{t('receipt-intro')}</p>
{#if form?.error}<Notice text={form.error} error />{/if}
<p><a data-sveltekit-reload href={`/stores/${data.order.merchant_id}/${data.order.store_id}/orders/${data.order.id}`}>{t('orders-refresh')}</a></p>
<OrderDetails order={data.order} />
{#if developmentActions(data.order, data.developmentPayments).length}
<section aria-label={t('payment-development')}>
  <h2>{t('payment-development')}</h2><p>{t('payment-development-help')}</p>
  {#each developmentActions(data.order, data.developmentPayments) as action}
    <form method="POST" action="?/payment">
      <input type="hidden" name="action" value={action} /><input type="hidden" name="revision" value={data.order.revision} />
      <input type="hidden" name="key" value={`${data.order.id}-${data.order.revision}-pay-${action}`} />
      <ActionButton icon={CreditCard} label={t(`payment-action-${action}`)} />
    </form>
  {/each}
</section>
{/if}
{#if canCancelOrder(data.order)}
  <form method="POST" action="?/cancel"><input type="hidden" name="revision" value={data.order.revision} />
    <input type="hidden" name="key" value={`${data.order.id}-${data.order.revision}-cancel`} />
    <ActionButton icon={X} label={t('orders-cancel')} danger />
  </form>
{/if}
<p>{t('receipt-cookie-help')}</p>
<a href={`/stores/${data.order.merchant_id}/${data.order.store_id}`}>{t('receipt-back')}</a>
