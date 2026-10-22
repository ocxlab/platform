<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { useI18n } from '$lib/i18n/context';
  import { liveUpdates, type LiveStatus } from '$lib/live-updates';
  import Radio from '@lucide/svelte/icons/radio';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  let { url, refresh }: { url: string; refresh: string } = $props();
  const { t } = useI18n();
  let status = $state<LiveStatus>('connecting');
  $effect(() => {
    const updates = liveUpdates(url, invalidateAll, value => { status = value; });
    const visibility = () => updates.visible(document.visibilityState === 'visible');
    visibility();
    document.addEventListener('visibilitychange', visibility);
    return () => { document.removeEventListener('visibilitychange', visibility); updates.close(); };
  });
</script>
<div class="live-controls">
  <span class="live-status" role="status"><Radio size={16} aria-hidden="true" />{t(`live-${status}`)}</span>
  <a data-sveltekit-reload href={refresh}><RefreshCw size={17} aria-hidden="true" />{t('orders-refresh')}</a>
</div>
<noscript><p>{t('live-noscript')}</p></noscript>
<style>
  .live-controls { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: .75rem; margin-block: 1rem; }
  .live-status { display: inline-flex; gap: .5rem; align-items: center; color: #526671; font-size: .9rem; }
  a { display: inline-flex; align-items: center; gap: .5rem; padding: .55rem .7rem; min-height: 44px; border: 1px solid #708692; border-radius: .3rem; text-decoration: none; background: white; }
</style>
