<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import Notice from '$lib/components/Notice.svelte';
  import CustomerReceipt from '$lib/components/CustomerReceipt.svelte';
  const { t } = useI18n();
  let { data, form } = $props();
</script>
<h1>{t('receipt-title')}</h1>
<p>{t('receipt-intro')}</p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#key data.order}
  <CustomerReceipt initialOrder={data.order} receipt={data.receipt} storefront={data.storefront} developmentPayments={data.developmentPayments} />
{/key}
