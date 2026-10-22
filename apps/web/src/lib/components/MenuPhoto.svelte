<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import { uploadMenuPhoto } from '$lib/media-upload';
  import Trash2 from '@lucide/svelte/icons/trash-2';
  import ActionButton from './ActionButton.svelte';
  const { t } = useI18n();
  let { imageId = $bindable(), endpoint, name, onbusy }: { imageId?: string; endpoint: string; name: string; onbusy?: (busy: boolean) => void } = $props();
  let uploading = $state(false);
  let failed = $state(false);
  async function upload(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0]; if (!file) return;
    uploading = true; failed = false; onbusy?.(true);
    try { imageId = await uploadMenuPhoto(endpoint, file); } catch { failed = true; }
    finally { uploading = false; input.value = ''; onbusy?.(false); }
  }
</script>
<div class="menu-photo">
  {#if imageId}<img src={`${endpoint}/${encodeURIComponent(imageId)}`} alt={name} loading="lazy" width="160" height="120" />{/if}
  <label>{t('media-upload')}<input type="file" accept="image/jpeg,image/png,image/webp" disabled={uploading} onchange={upload} /></label>
  <p class="muted">{t('media-help')}</p>
  {#if uploading}<p role="status">{t('media-uploading')}</p>{/if}
  {#if failed}<p role="alert">{t('media-failed')}</p>{/if}
  {#if imageId}<ActionButton type="button" icon={Trash2} label={t('media-remove')} disabled={uploading} onclick={() => imageId = undefined} />{/if}
</div>
<style>
  .menu-photo { margin-block: 1rem; }
  img { object-fit: cover; border-radius: .5rem; display: block; margin-block-end: .5rem; }
</style>
