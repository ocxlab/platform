<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import { merchantStorePath } from '$lib/merchant-paths';
  import Utensils from '@lucide/svelte/icons/utensils';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  let { store, active }: { store: { display_name: string; handle?: string }; active: 'menu' | 'orders' } = $props();
  const { t } = useI18n();
  const root = $derived(merchantStorePath(store));
</script>
<div class="workspace-heading">
  <h1><bdi dir="auto">{store.display_name}</bdi></h1>
  <a href="/merchant">{t('merchant-stores')}</a>
</div>
<nav aria-label={t('merchant-workspace-navigation')}>
  <a href={root} aria-current={active === 'menu' ? 'page' : undefined}><Utensils size={18} aria-hidden="true" />{t('merchant-menu-tab')}</a>
  <a href={`${root}/orders`} aria-current={active === 'orders' ? 'page' : undefined}><ClipboardList size={18} aria-hidden="true" />{t('merchant-orders-tab')}</a>
</nav>
<style>
  .workspace-heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
  .workspace-heading h1 { margin-block-end: .7rem; }
  nav { display: flex; gap: .5rem; border-block-end: 1px solid #ccd8de; margin-block-end: 1.2rem; }
  nav a { display: inline-flex; gap: .5rem; align-items: center; min-height: 44px; padding: .75rem 1rem; text-decoration: none; border-block-end: 3px solid transparent; }
  nav a[aria-current] { border-color: #20586f; background: #edf4f7; font-weight: 700; }
</style>
