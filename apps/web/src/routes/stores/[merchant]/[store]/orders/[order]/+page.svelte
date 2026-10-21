<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t } = useI18n();
  import OrderDetails from '$lib/components/OrderDetails.svelte';
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import X from '@lucide/svelte/icons/x';
  let { data, form } = $props();
</script>
<h1>{t('receipt-title')}</h1>
<p>{t('receipt-intro')}</p>
{#if form?.error}<Notice text={form.error} error />{/if}
<p><a href={`?revision=${data.order.revision}`}>{t('orders-refresh')}</a></p>
<OrderDetails order={data.order} />
{#if data.order.resolution === 'open' && data.order.payment === 'not_started' && data.order.fulfilment_progress === 'not_started'}
  <form method="POST" action="?/cancel"><input type="hidden" name="revision" value={data.order.revision} />
    <input type="hidden" name="key" value={`${data.order.id}-${data.order.revision}-cancel`} />
    <ActionButton icon={X} label={t('orders-cancel')} danger />
  </form>
{/if}
<p>{t('receipt-cookie-help')}</p>
<a href={`/stores/${data.order.merchant_id}/${data.order.store_id}`}>{t('receipt-back')}</a>
