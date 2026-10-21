<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t } = useI18n();
  import Notice from '$lib/components/Notice.svelte';
  import OrderDetails from '$lib/components/OrderDetails.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import LogIn from '@lucide/svelte/icons/log-in';
  import LogOut from '@lucide/svelte/icons/log-out';
  import Search from '@lucide/svelte/icons/search';
  import Eye from '@lucide/svelte/icons/eye';
  import EyeOff from '@lucide/svelte/icons/eye-off';
  let { data, form } = $props();
</script>
<h1>{t('admin-title')}</h1>
<p>{t('admin-intro')}</p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if form?.success}<Notice text={form.success} />{/if}
{#if !data.connected}
<section><h2>{t('admin-connect-heading')}</h2><p class="muted">{t('admin-connect-help')}</p><form method="POST" action="?/connect"><label>{t('admin-token')}<input type="password" name="token" required autocomplete="off" /></label><ActionButton icon={LogIn} label={t('admin-connect')} /></form></section>
{:else}
<section><h2>{t('orders-admin-heading')}</h2><p>{t('orders-admin-help')}</p>
<form method="POST" action="?/inspectOrder">
  <label>{t('orders-admin-merchant')}<input name="merchant" dir="ltr" required value={form?.order?.merchant_id ?? form?.publication?.merchant_id ?? ''} /></label>
  <label>{t('orders-admin-store')}<input name="store" dir="ltr" required value={form?.order?.store_id ?? form?.publication?.store_id ?? ''} /></label>
  <label>{t('orders-admin-reference')}<input name="order" dir="ltr" required value={form?.order?.id ?? ''} /></label>
  <ActionButton icon={Search} label={t('orders-admin-inspect')} />
</form></section>
{#if form?.order}<OrderDetails order={form.order} />{/if}
<section><h2>{t('admin-find')}</h2><form method="POST" action="?/inspect"><label>{t('merchant-id')}<input name="merchant" dir="ltr" required value={form?.publication?.merchant_id ?? ''} /></label><label>{t('admin-store-id')}<input name="store" dir="ltr" required value={form?.publication?.store_id ?? ''} /></label><ActionButton icon={Search} label={t('admin-inspect')} /></form></section>
{#if form?.publication}
<section><h2>{t('admin-visibility')}</h2><p>{t('admin-status', { generation: form.publication.generation, revision: form.publication.revision ?? t('merchant-none'), visibility: form.publication.suspended ? t('admin-suspended') : t('admin-not-suspended') })}</p>
<form method="POST" action="?/suspend"><input type="hidden" name="merchant" value={form.publication.merchant_id} /><input type="hidden" name="store" value={form.publication.store_id} /><input type="hidden" name="generation" value={form.publication.generation} /><input type="hidden" name="key" value={data.request_key} /><input type="hidden" name="suspended" value={String(!form.publication.suspended)} /><label>{t('admin-reason')}<textarea name="reason" dir="auto" required maxlength="500"></textarea></label><ActionButton icon={form.publication.suspended ? Eye : EyeOff} label={form.publication.suspended ? t('admin-restore') : t('admin-suspend')} /></form>
</section>
{/if}
<form method="POST" action="?/disconnect"><ActionButton icon={LogOut} label={t('admin-disconnect')} /></form>
{/if}
