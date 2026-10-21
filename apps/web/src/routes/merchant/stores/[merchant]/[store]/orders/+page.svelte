<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import { money } from '$lib/money';
  const { t, locale } = useI18n();
  let { data } = $props();
  const root = $derived(`/merchant/stores/${data.store.merchant_id}/${data.store.id}`);
</script>
<h1>{t('orders-title')}</h1>
<p><bdi dir="auto">{data.store.display_name}</bdi> · {t('orders-inbox-help')}</p>
<p><a href={root}>{t('orders-back')}</a> · <a data-sveltekit-reload href={`${root}/orders`}>{t('orders-refresh')}</a></p>
{#if !data.orders.items.length}<p>{t('orders-empty')}</p>{:else}
<div class="order-table"><table>
  <thead><tr><th>{t('receipt-reference')}</th><th>{t('orders-created')}</th><th>{t('orders-total')}</th><th>{t('orders-status')}</th></tr></thead>
  <tbody>{#each data.orders.items as order}<tr>
    <th scope="row"><a href={`${root}/orders/${order.id}`}><bdi dir="ltr">{order.id}</bdi></a></th>
    <td><time datetime={order.created_at}>{new Date(order.created_at).toLocaleString(locale())}</time></td>
    <td><bdi>{money(order.terms.total_minor, locale())}</bdi></td>
    <td>{order.resolution === 'cancelled' ? t('orders-event-cancelled') : order.resolution === 'rejected' ? t('orders-event-rejected') : order.acceptance === 'accepted' ? t('orders-event-accepted') : t('orders-awaiting')}</td>
  </tr>{/each}</tbody>
</table></div>
{/if}
{#if data.orders.next_after}<a href={`?after=${data.orders.next_after}`}>{t('orders-next')}</a>{/if}
<style>.order-table { overflow-x: auto; } th, td { text-align: start; padding: .65rem; } th bdi { overflow-wrap: anywhere; }</style>
