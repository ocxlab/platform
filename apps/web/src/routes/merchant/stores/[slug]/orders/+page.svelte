<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import { money } from '$lib/money';
  import MerchantStoreNav from '$lib/components/MerchantStoreNav.svelte';
  import { merchantStorePath } from '$lib/merchant-paths';
  import OrderLiveUpdates from '$lib/components/OrderLiveUpdates.svelte';
  const { t, locale } = useI18n();
  let { data } = $props();
  const root = $derived(merchantStorePath(data.store));
  const dateFormat = $derived(new Intl.DateTimeFormat(locale(), { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Europe/Berlin' }));
</script>
<MerchantStoreNav store={data.store} active="orders" />
<section aria-labelledby="orders-heading">
<div class="list-heading"><h2 id="orders-heading">{t('orders-title')}</h2>
</div>
<OrderLiveUpdates url={`${root}/orders/updates`} refresh={`${root}/orders`} />
<p class="muted">{t('orders-inbox-help')}</p>
{#if !data.orders.items.length}<p>{t('orders-empty')}</p>{:else}
<!-- svelte-ignore a11y_no_noninteractive_tabindex (Keyboard users need to focus and scroll the overflowing table.) -->
<div class="order-table" role="region" aria-label={t('orders-title')} tabindex="0"><table>
  <thead><tr><th scope="col">{t('receipt-reference')}</th><th scope="col">{t('orders-created')}</th><th scope="col" class="amount">{t('orders-total')}</th><th scope="col">{t('orders-status')}</th><th scope="col">{t('admin-payment')}</th><th scope="col">{t('fulfilment-title')}</th></tr></thead>
  <tbody>{#each data.orders.items as order (order.id)}<tr>
    <th scope="row"><a href={`${root}/orders/${order.id}`}><bdi dir="ltr">{order.reference ?? order.id}</bdi></a></th>
    <td><time datetime={order.created_at}>{dateFormat.format(new Date(order.created_at))}</time></td>
    <td class="amount"><bdi>{money(order.terms.total_minor, locale())}</bdi></td>
    <td><span class="badge">{t(`admin-state-${order.resolution === 'open' ? order.acceptance : order.resolution}`)}</span></td>
    <td>{t(`admin-payment-${order.payment}`)}</td>
    <td>{t(`admin-fulfilment-${order.fulfilment_progress}`)}</td>
  </tr>{/each}</tbody>
</table></div>
{/if}
{#if data.orders.next_after}<a href={`?after=${data.orders.next_after}`}>{t('orders-next')}</a>{/if}
</section>
<style>
  .list-heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
  .order-table { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  th, td { text-align: start; padding: .8rem .75rem; border-block-end: 1px solid #ccd8de; vertical-align: middle; }
  thead { background: #f1f6f8; color: #20586f; }
  tbody th { font-weight: 600; }
  tbody tr:hover { background: #f8fafb; }
  .amount { text-align: end; white-space: nowrap; }
  th bdi, time { white-space: nowrap; }
  .badge { display: inline-block; border: 1px solid #ccd8de; border-radius: .3rem; padding: .2rem .5rem; white-space: nowrap; }
  th a { display: inline-block; padding-block: .5rem; }
</style>
