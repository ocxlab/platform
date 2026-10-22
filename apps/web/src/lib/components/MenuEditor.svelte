<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t } = useI18n();
  import { untrack } from 'svelte';
  import Plus from '@lucide/svelte/icons/plus';
  import Trash2 from '@lucide/svelte/icons/trash-2';
  import ActionButton from './ActionButton.svelte';
  import MenuPhoto from './MenuPhoto.svelte';
  import type { components } from '$lib/api/generated';
  type Menu = components['schemas']['FoodMenu'];
  let { initial, base, mediaEndpoint, uploading = $bindable(false) }: { initial: Menu; base: number; mediaEndpoint?: string; uploading?: boolean } = $props();
  const activeUploads = new Set<string>();
  function uploadStatus(id: string, busy: boolean) {
    if (busy) activeUploads.add(id); else activeUploads.delete(id);
    uploading = activeUploads.size > 0;
  }
  let menu = $state<Menu>(untrack(() => structuredClone(initial)));
  const identifier = () => Array.from(crypto.getRandomValues(new Uint8Array(12)), (b) => b.toString(16).padStart(2, '0')).join('');
  const variant = () => ({ id: identifier(), name: t('editor-default-variant'), price_minor: 0 });
  function addItem() { menu.items.push({ id: identifier(), name: t('editor-default-item'), available: true, variants: [variant()], option_groups: [] }); }
  function removeItem(id: string) {
    menu.items = menu.items.filter((item) => item.id !== id);
  }
  function removeCategory(id: string) {
    menu.categories = (menu.categories ?? []).filter(category => category.id !== id);
    for (const item of menu.items) if (item.category_id === id) delete item.category_id;
  }
  function moveCategory(index: number, offset: number) {
    const categories = [...(menu.categories ?? [])];
    [categories[index], categories[index + offset]] = [categories[index + offset], categories[index]];
    menu.categories = categories;
  }
  function addOption(group: Menu['items'][number]['option_groups'][number]) {
    const allAllowed = group.max_selected === group.options.length;
    group.options.push({ id: identifier(), name: t('editor-default-option'), price_minor: 0 });
    if (allAllowed) group.max_selected = group.options.length;
  }
</script>
<input type="hidden" name="document" value={JSON.stringify(menu)} />
<input type="hidden" name="base" value={base} />
<p class="muted">{t('editor-removal-help')}</p>
<details class="editor-help"><summary>{t('editor-help-title')}</summary><p>{t('editor-help-variants')}</p><p>{t('editor-help-extras')}</p></details>
<fieldset class="category-editor"><legend>{t('menu-categories')}</legend>
  <p class="muted">{t('menu-category-help')}</p>
  {#each menu.categories ?? [] as category, index (category.id)}
    <div class="category-row">
      <label>{t('menu-category-name')}<input dir="auto" bind:value={category.name} required maxlength="80" /></label>
      <div class="category-actions">
        <button type="button" disabled={index === 0} aria-label={t('menu-move-up', { name: category.name })} onclick={() => moveCategory(index, -1)}>↑</button>
        <button type="button" disabled={index === (menu.categories?.length ?? 0) - 1} aria-label={t('menu-move-down', { name: category.name })} onclick={() => moveCategory(index, 1)}>↓</button>
        <ActionButton type="button" icon={Trash2} iconOnly danger label={t('menu-remove-category', { name: category.name })} onclick={() => removeCategory(category.id)} />
      </div>
    </div>
  {/each}
  <ActionButton type="button" icon={Plus} label={t('menu-add-category')} disabled={(menu.categories?.length ?? 0) >= 50} onclick={() => { menu.categories = [...(menu.categories ?? []), { id: identifier(), name: t('menu-new-category') }]; }} />
</fieldset>
{#each menu.items as item (item.id)}
  <fieldset><legend>{t('editor-item')}</legend>
    <label>{t('editor-name')}<input dir="auto" bind:value={item.name} required maxlength="160" /></label>
    <label>{t('menu-category')}<select value={item.category_id ?? ''} onchange={(event) => { const id = event.currentTarget.value; if (id) item.category_id = id; else delete item.category_id; }}><option value="">{t('menu-uncategorized')}</option>{#each menu.categories ?? [] as category}<option value={category.id} dir="auto">{category.name}</option>{/each}</select></label>
    <label><input type="checkbox" bind:checked={item.available} />{t('editor-available')}</label>
    {#if mediaEndpoint}<MenuPhoto bind:imageId={item.image_id} endpoint={mediaEndpoint} name={item.name} onbusy={(busy) => uploadStatus(item.id, busy)} />{/if}
    <h3>{t('editor-variants-heading')}</h3>
    <p class="muted">{t('editor-variants-help')}</p>
    {#each item.variants as choice (choice.id)}
      <div class="row"><label>{t('editor-variant')}<input dir="auto" bind:value={choice.name} required maxlength="80" /></label><label>{t('editor-variant-price')}<input type="number" min="0" step="0.01" max="10000000" required value={choice.price_minor / 100} oninput={(e) => choice.price_minor = Math.round(e.currentTarget.valueAsNumber * 100)} /></label></div>
    {/each}
    <ActionButton type="button" icon={Plus} label={t('editor-add-variant')} onclick={() => item.variants.push(variant())} disabled={item.variants.length >= 30} />
    <h3>{t('editor-extras-heading')}</h3>
    <p class="muted">{t('editor-extras-help')}</p>
    {#if !item.option_groups.length}<p class="extras-empty">{t('editor-no-extras')}</p>{/if}
    {#each item.option_groups as group (group.id)}
      <fieldset><legend>{t('editor-extras')}</legend>
        <label>{t('editor-group-name')}<input dir="auto" bind:value={group.name} required maxlength="80" /></label>
        <details class="choice-rules"><summary>{t('editor-choice-rules')}</summary><p class="muted">{t('editor-choice-rules-help')}</p><div class="row"><label>{t('editor-min')}<input type="number" min="0" max="30" bind:value={group.min_selected} required /></label><label>{t('editor-max')}<input type="number" min="1" max="30" bind:value={group.max_selected} required /></label></div></details>
        {#each group.options as option (option.id)}
          <div class="row"><label>{t('editor-option')}<input dir="auto" bind:value={option.name} required maxlength="80" /></label><label>{t('editor-option-price')}<input type="number" min="0" step="0.01" max="10000000" required value={option.price_minor / 100} oninput={(e) => option.price_minor = Math.round(e.currentTarget.valueAsNumber * 100)} /></label></div>
        {/each}
        <ActionButton type="button" icon={Plus} label={t('editor-add-option')} onclick={() => addOption(group)} disabled={group.options.length >= 30} />
        <ActionButton type="button" icon={Trash2} label={t('editor-remove-group', { name: group.name })} onclick={() => item.option_groups = item.option_groups.filter(candidate => candidate.id !== group.id)} />
      </fieldset>
    {/each}
    <ActionButton type="button" icon={Plus} label={t('editor-add-group')} onclick={() => item.option_groups.push({ id: identifier(), name: t('editor-default-group'), min_selected: 0, max_selected: 1, options: [{ id: identifier(), name: t('editor-default-extra'), price_minor: 0 }] })} disabled={item.option_groups.length >= 20} />
    <ActionButton type="button" icon={Trash2} iconOnly danger label={t('editor-remove', { name: item.name })} disabled={uploading} onclick={() => removeItem(item.id)} />
  </fieldset>
{/each}
{#if menu.items.length === 0}
  <p>{t('editor-empty')}</p>
{/if}
<ActionButton type="button" icon={Plus} label={t('editor-add-item')} onclick={addItem} disabled={menu.items.length >= 200} />
<style>
  .category-row { display: flex; align-items: end; flex-wrap: wrap; gap: .75rem; }
  .category-row label { flex: 1; min-inline-size: 10rem; }
  .category-actions { display: flex; align-items: center; gap: .4rem; margin-block-end: 1rem; }
  .category-actions button { margin: 0; }
  h3 { margin-block: 1.5rem .35rem; }
  h3 + p { margin-block-start: .35rem; }
  .editor-help { border-inline-start: 3px solid #20586f; padding: 1rem 1.2rem; background: #edf4f7; border-radius: .4rem; }
  summary { cursor: pointer; font-weight: 600; color: #20586f; min-height: 2rem; }
  .choice-rules { margin-block: .75rem; }
  .extras-empty { padding: .75rem 1rem; border: 1px dashed #93a8b3; border-radius: .35rem; color: #526671; }
</style>
