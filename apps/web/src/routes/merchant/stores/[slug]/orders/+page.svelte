<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import { money } from '$lib/money';
  import MerchantStoreNav from '$lib/components/MerchantStoreNav.svelte';
  import { merchantStorePath } from '$lib/merchant-paths';
  const { t, locale } = useI18n();
  let { data } = $props();
  const root = $derived(merchantStorePath(data.store));
</script>
<MerchantStoreNav store={data.store} active="orders" />
<h2>{t('orders-title')}</h2>
<p>{t('orders-inbox-help')}</p>
<p><a href={root}>{t('orders-back')}</a> · <a data-sveltekit-reload href={`${root}/orders`}>{t('orders-refresh')}</a></p>
{#if !data.orders.items.length}<p>{t('orders-empty')}</p>{:else}
<div class="order-table"><table>
  <thead><tr><th>{t('receipt-reference')}</th><th>{t('orders-created')}</th><th>{t('orders-total')}</th><th>{t('orders-status')}</th></tr></thead>
  <tbody>{#each data.orders.items as order}<tr>
    <th scope="row"><a href={`${root}/orders/${order.id}`}><bdi dir="ltr">{order.reference ?? order.id}</bdi></a></th>
    <td><time datetime={order.created_at}>{new Date(order.created_at).toLocaleString(locale())}</time></td>
    <td><bdi>{money(order.terms.total_minor, locale())}</bdi></td>
    <td>{order.resolution === 'cancelled' ? t('orders-event-cancelled') : order.resolution === 'rejected' ? t('orders-event-rejected') : order.acceptance === 'accepted' ? t('orders-event-accepted') : t('orders-awaiting')}<br /><small>{t(`payment-status-${order.payment}`)}</small>
      {#if order.fulfilment_progress !== 'not_started'}<br /><small>{t(`fulfilment-status-${order.fulfilment_progress}`)}</small>{/if}
    </td>
  </tr>{/each}</tbody>
</table></div>
{/if}
{#if data.orders.next_after}<a href={`?after=${data.orders.next_after}`}>{t('orders-next')}</a>{/if}
<style>.order-table { overflow-x: auto; } th, td { text-align: start; padding: .65rem; } th bdi { overflow-wrap: anywhere; }</style>
