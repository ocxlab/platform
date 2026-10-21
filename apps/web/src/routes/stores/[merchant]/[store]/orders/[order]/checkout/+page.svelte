<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import { money } from '$lib/money';
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import CreditCard from '@lucide/svelte/icons/credit-card';
  const { t, locale } = useI18n();
  let { data, form } = $props();
</script>
<h1>{t('payment-checkout-title')}</h1>
<p>{t('payment-development-help')}</p>
{#if form?.error}<Notice text={form.error} error />{/if}
<section aria-label={t('payment-checkout-title')}>
  <h2>{t('storefront-total', { amount: money(data.order.terms.total_minor, locale()) })}</h2>
  <p>{t('receipt-reference')} <bdi dir="ltr">{data.order.reference ?? data.order.id}</bdi></p>
  <p>{t('payment-checkout-help')}</p>
  <div class="checkout-actions">
    {#each ['succeed', 'decline', 'abandon'] as action}
      <form method="POST">
        <input type="hidden" name="action" value={action} />
        <input type="hidden" name="revision" value={data.order.revision} />
        <input type="hidden" name="key" value={`${data.order.id}-${data.order.revision}-pay-${action}`} />
        <ActionButton icon={CreditCard} label={t(`payment-action-${action}`)} />
      </form>
    {/each}
  </div>
  <a data-sveltekit-reload href={data.receipt}>{t('payment-return')}</a>
</section>
<style>
  .checkout-actions { display: flex; flex-wrap: wrap; gap: .75rem; margin-block: 1rem; }
  .checkout-actions form { margin: 0; }
</style>
