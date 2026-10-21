<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t, locale } = useI18n();
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import LogIn from '@lucide/svelte/icons/log-in';
  import LogOut from '@lucide/svelte/icons/log-out';
  import Plus from '@lucide/svelte/icons/plus';
  let { data, form } = $props();
  const dateFormat = $derived(new Intl.DateTimeFormat(locale(), { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Europe/Berlin' }));
</script>
<h1>{t('merchant-title')}</h1>
<p>{t('merchant-intro')}</p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if data.stores_error}<Notice text={data.stores_error} error />{/if}
{#if !data.merchant}
  <section><h2>{t('merchant-connect-heading')}</h2><p class="muted">{t('merchant-connect-help')}</p>
  <form method="POST" action="?/connect"><label>{t('merchant-id')}<input name="merchant" dir="ltr" required autocomplete="off" /></label><label>{t('merchant-token')}<input name="token" type="password" required autocomplete="off" /></label><ActionButton icon={LogIn} label={t('merchant-connect')} /></form></section>
{:else}
  <section><h2>{t('merchant-create-heading')}</h2><form method="POST" action="?/create"><input type="hidden" name="key" value={data.request_key} /><label>{t('merchant-store-name')}<input name="name" dir="auto" required maxlength="160" /></label><ActionButton icon={Plus} label={t('merchant-create')} /></form></section>
  <section aria-labelledby="managed-stores"><h2 id="managed-stores">{t('merchant-stores')}</h2>
    {#if data.stores.length}
      <!-- svelte-ignore a11y_no_noninteractive_tabindex (Keyboard users need to scroll the table on narrow screens.) -->
      <div class="store-table" role="region" aria-label={t('merchant-store-table')} tabindex="0">
        <table>
          <caption>{t('merchant-table-caption')}</caption>
          <thead><tr><th scope="col">{t('merchant-store-name')}</th><th scope="col">{t('merchant-menu-count')}</th><th scope="col">{t('merchant-draft-revision')}</th><th scope="col">{t('merchant-updated')}</th></tr></thead>
          <tbody>{#each data.stores as store (store.id)}
            <tr>
              <th scope="row"><a href={`/merchant/stores/${data.merchant}/${store.id}`}><bdi dir="auto">{store.display_name}</bdi></a></th>
              <td>{store.menu_item_count ?? '—'}</td>
              <td>{store.draft_revision || t('merchant-no-draft')}</td>
              <td><time datetime={store.updated_at}>{dateFormat.format(new Date(store.updated_at))}</time></td>
            </tr>
          {/each}</tbody>
        </table>
      </div>
    {:else if !data.stores_error}
      <p>{data.after ? t('merchant-no-more') : t('merchant-no-stores')}</p>
    {/if}
    <nav aria-label={t('merchant-store-pages')}>
      {#if data.after}<a href="/merchant">{t('merchant-first-page')}</a>{/if}
      {#if data.next_after}<a href={`/merchant?after=${data.next_after}`}>{t('merchant-next-stores')}</a>{/if}
    </nav>
  </section>
  <form method="POST" action="?/disconnect"><ActionButton icon={LogOut} label={t('merchant-disconnect')} /></form>
{/if}
<style>
  .store-table { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; text-align: start; }
  caption { text-align: start; color: #52636d; font-size: .9rem; padding-bottom: .75rem; }
  th, td { text-align: start; padding: .75rem; border-bottom: 1px solid #ccd8de; }
  thead { background: #f1f6f8; color: #20586f; }
  tbody th { font-weight: 500; }
  td:last-child { white-space: nowrap; }
</style>
