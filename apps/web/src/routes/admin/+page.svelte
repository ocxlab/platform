<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import Notice from '$lib/components/Notice.svelte';
  import OrderDetails from '$lib/components/OrderDetails.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import LogIn from '@lucide/svelte/icons/log-in';
  import LogOut from '@lucide/svelte/icons/log-out';
  import Search from '@lucide/svelte/icons/search';
  import Eye from '@lucide/svelte/icons/eye';
  import EyeOff from '@lucide/svelte/icons/eye-off';
  import Store from '@lucide/svelte/icons/store';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import FilterX from '@lucide/svelte/icons/filter-x';
  const { t, locale } = useI18n();
  let { data, form } = $props();
  const dateFormat = $derived(new Intl.DateTimeFormat(locale(), { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Europe/Berlin' }));
  const statuses = $derived(data.tab === 'stores' ? ['unpublished', 'published', 'suspended'] : ['pending', 'accepted', 'rejected', 'cancelled']);
  const action = (name: string) => `?${data.list_query}&/${name}#admin-details`;
</script>
<h1>{t('admin-title')}</h1>
<p>{t('admin-intro')}</p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if form?.success}<Notice text={form.success} />{/if}
{#if !data.connected}
  <section><h2>{t('admin-connect-heading')}</h2><p class="muted">{t('admin-connect-help')}</p><form method="POST" action="?/connect"><label>{t('admin-token')}<input type="password" name="token" required autocomplete="off" /></label><ActionButton icon={LogIn} label={t('admin-connect')} /></form></section>
{:else}
  <div class="admin-toolbar">
    <nav class="admin-tabs" aria-label={t('admin-navigation')}>
      <a href="/admin?tab=stores" aria-current={data.tab === 'stores' ? 'page' : undefined}><Store size={18} aria-hidden="true" />{t('admin-stores')}</a>
      <a href="/admin?tab=orders" aria-current={data.tab === 'orders' ? 'page' : undefined}><ClipboardList size={18} aria-hidden="true" />{t('admin-orders')}</a>
    </nav>
    <form method="POST" action="?/disconnect"><ActionButton icon={LogOut} label={t('admin-disconnect')} iconOnly /></form>
  </div>
  <section aria-labelledby="admin-list-heading">
    <div class="list-heading"><h2 id="admin-list-heading">{t(data.tab === 'stores' ? 'admin-stores' : 'admin-orders')}</h2><a class="secondary-control" href={data.refresh_url} data-sveltekit-reload><RefreshCw size={18} aria-hidden="true" />{t('orders-refresh')}</a></div>
    <form method="GET" action="/admin" class="admin-filters">
      <input type="hidden" name="tab" value={data.tab} />
      <label>{t('admin-search')}<input type="search" name="q" value={data.q} maxlength="160" dir="auto" placeholder={t(data.tab === 'stores' ? 'admin-search-stores' : 'admin-search-orders')} /></label>
      <label>{t('admin-filter-status')}<select name="status" value={data.status}><option value="">{t('admin-all-statuses')}</option>{#each statuses as status}<option value={status}>{t(`admin-state-${status}`)}</option>{/each}</select></label>
      <label>{t('admin-page-size')}<select name="limit" value={String(data.limit)}>{#each [10, 25, 50] as size}<option value={String(size)}>{size}</option>{/each}</select></label>
      <div class="filter-actions"><ActionButton icon={Search} label={t('ui-search')} />
      <a class="secondary-control" href={`/admin?tab=${data.tab}`}><FilterX size={18} aria-hidden="true" />{t('admin-clear')}</a></div>
    </form>
    {#if data.list_error}<Notice text={data.list_error} error />
    {:else if data.stores.length || data.orders.length}
      <!-- svelte-ignore a11y_no_noninteractive_tabindex (Keyboard users need horizontal scrolling on narrow screens.) -->
      <div class="admin-table" role="region" aria-label={t(data.tab === 'stores' ? 'admin-stores' : 'admin-orders')} tabindex="0">
        <table>
          <caption>{t('admin-table-help')}</caption>
          {#if data.tab === 'stores'}
            <thead><tr><th scope="col">{t('merchant-store-name')}</th><th scope="col">{t('admin-merchant')}</th><th scope="col">{t('orders-status')}</th><th scope="col">{t('merchant-draft-revision')}</th><th scope="col">{t('merchant-updated')}</th><th scope="col">{t('admin-actions')}</th></tr></thead>
            <tbody>{#each data.stores as store (store.id)}<tr>
              <th scope="row"><bdi dir="auto">{store.display_name}</bdi><br /><small class="muted"><bdi dir="ltr">{store.handle}</bdi></small></th>
              <td><bdi dir="auto">{store.merchant_name}</bdi></td><td><span class="badge">{t(`admin-state-${store.publication_status}`)}</span></td><td>{store.draft_revision || t('merchant-no-draft')}</td><td><time datetime={store.updated_at}>{dateFormat.format(new Date(store.updated_at))}</time></td>
              <td><form method="POST" action={action('inspect')}><input type="hidden" name="merchant" value={store.merchant_id} /><input type="hidden" name="store" value={store.id} /><ActionButton icon={Eye} label={t('admin-inspect')} iconOnly /></form></td>
            </tr>{/each}</tbody>
          {:else}
            <thead><tr><th scope="col">{t('receipt-reference')}</th><th scope="col">{t('merchant-store-name')}</th><th scope="col">{t('orders-created')}</th><th scope="col">{t('orders-status')}</th><th scope="col">{t('admin-payment')}</th><th scope="col">{t('fulfilment-title')}</th><th scope="col">{t('admin-actions')}</th></tr></thead>
            <tbody>{#each data.orders as order (order.id)}<tr>
              <th scope="row"><bdi dir="ltr">{order.reference}</bdi></th><td><bdi dir="auto">{order.store_name}</bdi></td><td><time datetime={order.created_at}>{dateFormat.format(new Date(order.created_at))}</time></td>
              <td><span class="badge">{t(`admin-state-${order.resolution === 'open' ? order.acceptance : order.resolution}`)}</span></td><td>{t(`admin-payment-${order.payment}`)}</td><td>{t(`admin-fulfilment-${order.fulfilment_progress}`)}</td>
              <td><form method="POST" action={action('inspectOrder')}><input type="hidden" name="merchant" value={order.merchant_id} /><input type="hidden" name="store" value={order.store_id} /><input type="hidden" name="order" value={order.id} /><ActionButton icon={Eye} label={t('orders-admin-inspect')} iconOnly /></form></td>
            </tr>{/each}</tbody>
          {/if}
        </table>
      </div>
    {:else}<p role="status">{t('admin-empty')}</p>{/if}
    <nav class="pagination" aria-label={t('admin-pages')}>
      {#if data.paged}<a href={data.first_url}>{t('merchant-first-page')}</a>{/if}
      {#if data.previous_url}<a href={data.previous_url}>{t('admin-previous')}</a>{/if}
      {#if data.next_url}<a href={data.next_url}>{t('admin-next')}</a>{/if}
    </nav>
  </section>
  <div id="admin-details">
    {#if form?.order}<OrderDetails order={form.order} />{/if}
    {#if form?.publication}
      <section><h2>{t('admin-visibility')}</h2><p>{t('admin-status', { generation: form.publication.generation, revision: form.publication.revision ?? t('merchant-none'), visibility: form.publication.suspended ? t('admin-suspended') : t('admin-not-suspended') })}</p>
      <form method="POST" action={action('suspend')}><input type="hidden" name="merchant" value={form.publication.merchant_id} /><input type="hidden" name="store" value={form.publication.store_id} /><input type="hidden" name="generation" value={form.publication.generation} /><input type="hidden" name="key" value={data.request_key} /><input type="hidden" name="suspended" value={String(!form.publication.suspended)} /><label>{t('admin-reason')}<textarea name="reason" dir="auto" required maxlength="500"></textarea></label><ActionButton icon={form.publication.suspended ? Eye : EyeOff} label={form.publication.suspended ? t('admin-restore') : t('admin-suspend')} /></form></section>
    {/if}
  </div>
{/if}
<style>
  .admin-toolbar, .list-heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
  .admin-tabs { display: flex; gap: .5rem; }
  .admin-tabs a { display: inline-flex; gap: .5rem; align-items: center; }
  .admin-tabs a { padding: .75rem 1rem; border-block-end: 3px solid transparent; text-decoration: none; }
  .admin-tabs a[aria-current] { border-color: #20586f; background: #edf4f7; font-weight: 700; }
  .admin-filters { display: grid; grid-template-columns: minmax(12rem, 1fr) auto auto; gap: 1rem; align-items: end; margin-block: 1rem; }
  .admin-filters label { margin: 0; min-width: 0; }
  .admin-filters input, .admin-filters select { min-height: 44px; box-sizing: border-box; }
  .filter-actions { grid-column: 1 / -1; display: flex; align-items: center; gap: .75rem; flex-wrap: wrap; }
  .filter-actions :global(button) { margin: 0; }
  .filter-actions :global(.action) { display: inline-flex; }
  .secondary-control { display: inline-flex; align-items: center; justify-content: center; gap: .5rem; box-sizing: border-box; min-height: 44px; padding: .55rem .7rem; border: 1px solid #708692; border-radius: .3rem; background: #fff; text-decoration: none; }
  .secondary-control:hover { background: #edf4f7; }
  @media (max-width: 600px) { .admin-filters { grid-template-columns: 1fr 1fr; } .admin-filters label:first-of-type { grid-column: 1 / -1; } }
  .admin-table { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  th, td { text-align: start; padding: .75rem; border-block-end: 1px solid #ccd8de; vertical-align: middle; }
  thead { background: #f1f6f8; color: #20586f; }
  tbody th { font-weight: 500; }
  caption { text-align: start; font-size: .9rem; color: #52636d; padding-block-end: .75rem; }
  .badge { display: inline-block; border: 1px solid #ccd8de; border-radius: .3rem; padding: .15rem .45rem; white-space: nowrap; }
  .pagination { display: flex; gap: 1rem; margin-block-start: 1rem; }
  time { white-space: nowrap; }
  #admin-details { scroll-margin-block-start: 1rem; }
</style>
