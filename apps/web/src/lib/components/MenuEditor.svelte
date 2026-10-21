<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t } = useI18n();
  import { untrack } from 'svelte';
  import Plus from '@lucide/svelte/icons/plus';
  import Trash2 from '@lucide/svelte/icons/trash-2';
  import ActionButton from './ActionButton.svelte';
  import type { components } from '$lib/api/generated';
  type Menu = components['schemas']['FoodMenu'];
  let { initial, base }: { initial: Menu; base: number } = $props();
  let menu = $state<Menu>(untrack(() => structuredClone(initial)));
  const identifier = () => Array.from(crypto.getRandomValues(new Uint8Array(12)), (b) => b.toString(16).padStart(2, '0')).join('');
  const variant = () => ({ id: identifier(), name: t('editor-default-variant'), price_minor: 0 });
  function addItem() { menu.items.push({ id: identifier(), name: t('editor-default-item'), available: true, variants: [variant()], option_groups: [] }); }
  function removeItem(id: string) {
    menu.items = menu.items.filter((item) => item.id !== id);
  }
</script>
<input type="hidden" name="document" value={JSON.stringify(menu)} />
<input type="hidden" name="base" value={base} />
<p class="muted">{t('editor-removal-help')}</p>
{#each menu.items as item (item.id)}
  <fieldset><legend>{t('editor-item')}</legend>
    <label>{t('editor-name')}<input dir="auto" bind:value={item.name} required maxlength="160" /></label>
    <label><input type="checkbox" bind:checked={item.available} />{t('editor-available')}</label>
    {#each item.variants as choice (choice.id)}
      <div class="row"><label>{t('editor-variant')}<input dir="auto" bind:value={choice.name} required maxlength="80" /></label><label>{t('editor-variant-price')}<input type="number" min="0" step="0.01" max="10000000" required value={choice.price_minor / 100} oninput={(e) => choice.price_minor = Math.round(e.currentTarget.valueAsNumber * 100)} /></label></div>
    {/each}
    <ActionButton type="button" icon={Plus} label={t('editor-add-variant')} onclick={() => item.variants.push(variant())} disabled={item.variants.length >= 30} />
    {#each item.option_groups as group (group.id)}
      <fieldset><legend>{t('editor-extras')}</legend>
        <label>{t('editor-group-name')}<input dir="auto" bind:value={group.name} required maxlength="80" /></label>
        <div class="row"><label>{t('editor-min')}<input type="number" min="0" max="30" bind:value={group.min_selected} required /></label><label>{t('editor-max')}<input type="number" min="1" max="30" bind:value={group.max_selected} required /></label></div>
        {#each group.options as option (option.id)}
          <div class="row"><label>{t('editor-option')}<input dir="auto" bind:value={option.name} required maxlength="80" /></label><label>{t('editor-option-price')}<input type="number" min="0" step="0.01" max="10000000" required value={option.price_minor / 100} oninput={(e) => option.price_minor = Math.round(e.currentTarget.valueAsNumber * 100)} /></label></div>
        {/each}
        <ActionButton type="button" icon={Plus} label={t('editor-add-option')} onclick={() => group.options.push({ id: identifier(), name: t('editor-default-option'), price_minor: 0 })} disabled={group.options.length >= 30} />
      </fieldset>
    {/each}
    <ActionButton type="button" icon={Plus} label={t('editor-add-group')} onclick={() => item.option_groups.push({ id: identifier(), name: t('editor-default-group'), min_selected: 0, max_selected: 1, options: [{ id: identifier(), name: t('editor-default-extra'), price_minor: 0 }] })} disabled={item.option_groups.length >= 20} />
    <ActionButton type="button" icon={Trash2} iconOnly danger label={t('editor-remove', { name: item.name })} onclick={() => removeItem(item.id)} />
  </fieldset>
{/each}
{#if menu.items.length === 0}
  <p>{t('editor-empty')}</p>
{/if}
<ActionButton type="button" icon={Plus} label={t('editor-add-item')} onclick={addItem} disabled={menu.items.length >= 200} />
