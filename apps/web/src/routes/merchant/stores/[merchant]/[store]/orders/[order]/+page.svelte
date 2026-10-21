<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import OrderDetails from '$lib/components/OrderDetails.svelte';
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Check from '@lucide/svelte/icons/check';
  import X from '@lucide/svelte/icons/x';
  const { t } = useI18n();
  let { data, form } = $props();
</script>
<h1>{t('orders-order')}</h1>
<p><a href={`/merchant/stores/${data.order.merchant_id}/${data.order.store_id}/orders`}>{t('orders-inbox')}</a> · <a href={`?revision=${data.order.revision}`}>{t('orders-refresh')}</a></p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if form?.success}<Notice text={form.success} />{/if}
<OrderDetails order={data.order} />
{#if data.order.resolution === 'open' && data.order.acceptance === 'pending'}
<section><h2>{t('orders-decision')}</h2><p>{t('orders-decision-help')}</p>
  {#each ['accept', 'reject'] as decision}
    <form method="POST" action="?/decide">
      <input type="hidden" name="decision" value={decision} /><input type="hidden" name="revision" value={data.order.revision} />
      <input type="hidden" name="key" value={`${data.order.id}-${data.order.revision}-${decision}`} />
      <ActionButton icon={decision === 'accept' ? Check : X} label={decision === 'accept' ? t('orders-accept') : t('orders-reject')} danger={decision === 'reject'} />
    </form>
  {/each}
</section>
{/if}
