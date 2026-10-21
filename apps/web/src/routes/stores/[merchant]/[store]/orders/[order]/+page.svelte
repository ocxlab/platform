<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t, locale } = useI18n();
  import { money } from '$lib/money';
  let { data } = $props();
</script>
<h1>{t('receipt-title')}</h1>
<p>{t('receipt-intro')}</p>
<section aria-label={t('receipt-details')}>
  <h2>{t('receipt-selection')}</h2>
  {#each data.order.terms.lines as line}
    <p>{line.selection.quantity} × <bdi dir="auto">{line.item_name}</bdi> (<bdi dir="auto">{line.variant_name}</bdi>) · <bdi>{money(line.total_minor, locale())}</bdi>
    {#each line.selected_options ?? [] as option}<br /><small><bdi dir="auto">{option.group_name}</bdi>: <bdi dir="auto">{option.option_name}</bdi></small>{/each}</p>
  {/each}
  <p><strong>{t('storefront-total', { amount: money(data.order.terms.total_minor, locale()) })}</strong></p>
  <p>{t('receipt-progress')}</p>
  <p class="muted">{t('receipt-reference')} <bdi dir="ltr">{data.order.id}</bdi></p>
</section>
<p>{t('receipt-cookie-help')}</p>
<a href={`/stores/${data.order.merchant_id}/${data.order.store_id}`}>{t('receipt-back')}</a>
