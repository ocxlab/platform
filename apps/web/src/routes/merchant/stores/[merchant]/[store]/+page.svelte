<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t } = useI18n();
  import Notice from '$lib/components/Notice.svelte';
  import MenuEditor from '$lib/components/MenuEditor.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Upload from '@lucide/svelte/icons/upload';
  import EyeOff from '@lucide/svelte/icons/eye-off';
  import Save from '@lucide/svelte/icons/save';
  let { data, form } = $props();
</script>
<h1><bdi dir="auto">{data.store.display_name}</bdi></h1>
<p>{t('merchant-edit-intro')}</p>
<p><a href={`/merchant/stores/${data.store.merchant_id}/${data.store.id}/orders`}>{t('orders-inbox')}</a></p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if form?.success}<Notice text={form.success} />{/if}
<section><h2>{t('merchant-publication')}</h2>
  <p>{t('merchant-revisions', { published: data.publication.revision ?? t('merchant-none'), draft: data.draft?.revision ?? t('merchant-none') })}</p>
  {#if data.publication.suspended}<Notice text={t('merchant-suspended')} error />{/if}
  <p><a href={`/stores/${data.store.merchant_id}/${data.store.id}`}>{t('merchant-open-storefront')}</a></p>
  <form method="POST" action="?/publish"><input type="hidden" name="key" value={data.request_key + '-publish'} /><input type="hidden" name="generation" value={data.publication.generation} /><input type="hidden" name="revision" value={data.draft?.revision ?? ''} /><ActionButton icon={Upload} label={t('merchant-publish')} disabled={!data.draft} /></form>
  <form method="POST" action="?/publish"><input type="hidden" name="key" value={data.request_key + '-withdraw'} /><input type="hidden" name="generation" value={data.publication.generation} /><ActionButton icon={EyeOff} label={t('merchant-withdraw')} disabled={!data.publication.revision} /></form>
</section>
<section><h2>{t('merchant-edit-menu')}</h2><p>{t('merchant-price-help')}</p>
<form method="POST" action="?/save"><input type="hidden" name="key" value={data.request_key + '-draft'} />
  {#key data.draft?.revision}<MenuEditor initial={data.menu} base={data.draft?.revision ?? 0} />{/key}
  <ActionButton icon={Save} label={t('merchant-save')} />
</form></section>
