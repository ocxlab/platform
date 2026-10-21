<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Search from '@lucide/svelte/icons/search';
  let { data } = $props();
  const { t } = useI18n();
</script>
<h1>{t('ui-home-title')}</h1>
<p>{t('ui-home-intro')}</p>
<section><h2>{t('ui-for-merchants')}</h2><p>{t('ui-home-merchant')}</p><a href="/merchant">{t('ui-open-workspace')}</a></section>
<section id="stores" aria-labelledby="browse-heading">
  <h2 id="browse-heading">{t('ui-browse-store')}</h2><p>{t('ui-home-browse')}</p>
  <form method="GET" action="/#stores">
    <label>{t('ui-store-search')}<input type="search" name="q" value={data.query} maxlength="160" dir="auto" /></label>
    <ActionButton icon={Search} label={t('ui-search')} />
    {#if data.query}<a href="/#stores">{t('ui-show-all-stores')}</a>{/if}
  </form>
  {#if data.stores_error}<Notice text={data.stores_error} error />
  {:else if data.stores.length}
    <div class="store-list" role="region" aria-label={t('ui-published-stores')}>
      <ul>{#each data.stores as store (`${store.merchant_id}/${store.id}`)}
        <li><a href={`/stores/${store.merchant_id}/${store.id}`}><bdi class="store-name" dir="auto">{store.display_name}</bdi></a></li>
      {/each}</ul>
    </div>
    {#if data.next}<p><a href={data.next}>{t('ui-more-stores')}</a></p>{/if}
  {:else}<p role="status">{data.query ? t('ui-no-matching-stores') : t('ui-no-public-stores')}</p>{/if}
</section>

<style>
  .store-list { max-block-size: 22rem; overflow-y: auto; border: 1px solid var(--border, #d3dce0); border-radius: .5rem; }
  ul { list-style: none; margin: 0; padding: .25rem; }
  li + li { border-block-start: 1px solid var(--border, #d3dce0); }
  li a { display: block; padding: .9rem; overflow-wrap: anywhere; border-radius: .25rem; text-decoration: none; }
  li a:hover { background: #e5eff3; }
  .store-name { font-weight: 600; }
</style>
