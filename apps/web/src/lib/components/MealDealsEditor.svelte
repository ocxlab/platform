<script lang="ts">
  import type { FoodMenu } from '$lib/food-menu';
  import { useI18n } from '$lib/i18n/context';
  import ActionButton from './ActionButton.svelte';
  import MenuTranslations from './MenuTranslations.svelte';
  import Plus from '@lucide/svelte/icons/plus';
  import Trash2 from '@lucide/svelte/icons/trash-2';
  const { t } = useI18n();
  let { menu = $bindable() }: { menu: FoodMenu } = $props();
  function add() {
    const id = crypto.randomUUID();
    menu.meal_deals = [...(menu.meal_deals ?? []), { id, name: t('meal-new'), surcharge_minor: 100, main_item_ids: [], addon_choices: [] }];
  }
</script>
<fieldset class="meal-editor"><legend>{t('meal-title')}</legend>
  <p class="muted">{t('meal-help')}</p>
  {#each menu.meal_deals ?? [] as deal (deal.id)}
    <fieldset><legend><bdi dir="auto">{deal.name}</bdi></legend>
      <div class="row"><label>{t('meal-name')}<input dir="auto" bind:value={deal.name} required maxlength="160" /></label>
      <label>{t('meal-surcharge')}<input type="number" min="0" max="10000000" step="0.01" required value={deal.surcharge_minor / 100} oninput={event => deal.surcharge_minor = Math.round(event.currentTarget.valueAsNumber * 100)} /></label></div>
      <MenuTranslations bind:value={deal.translations} name={deal.name} />
      <details open={!deal.main_item_ids.length}><summary>{t('meal-mains')} · {deal.main_item_ids.length}</summary><div class="products">
      {#each menu.items as item}<label><input type="checkbox" checked={deal.main_item_ids.includes(item.id)} onchange={event => deal.main_item_ids = event.currentTarget.checked ? [...deal.main_item_ids, item.id] : deal.main_item_ids.filter(id => id !== item.id)} /><bdi dir="auto">{item.name}</bdi></label>{/each}
      </div></details>
      <details open={!deal.addon_choices.length}><summary>{t('meal-addons')} · {deal.addon_choices.length}</summary><p class="muted">{t('meal-addon-help')}</p><div class="products">
      {#each menu.items.filter(item => item.option_groups.every(group => group.min_selected === 0)) as item}{#each item.variants as variant}
        <label><input type="checkbox" checked={deal.addon_choices.some(choice => choice.item_id === item.id && choice.variant_id === variant.id)} onchange={event => deal.addon_choices = event.currentTarget.checked ? [...deal.addon_choices, { item_id: item.id, variant_id: variant.id }] : deal.addon_choices.filter(choice => choice.item_id !== item.id || choice.variant_id !== variant.id)} /><bdi dir="auto">{item.name} · {variant.name}</bdi></label>
      {/each}{/each}</div></details>
      {#if !deal.main_item_ids.length || !deal.addon_choices.length}<p class="required" role="status">{t('meal-required')}</p>{/if}
      <ActionButton type="button" icon={Trash2} danger label={t('meal-remove', { name: deal.name })} onclick={() => menu.meal_deals = menu.meal_deals?.filter(value => value.id !== deal.id)} />
    </fieldset>
  {/each}
  <ActionButton type="button" icon={Plus} label={t('meal-add')} disabled={(menu.meal_deals?.length ?? 0) >= 50 || !menu.items.length} onclick={add} />
</fieldset>
<style>
  .meal-editor { margin-block-start: 1.5rem; }
  details { margin-block: 1rem; } summary { cursor: pointer; color: #20586f; font-weight: 600; }
  .products { display: grid; grid-template-columns: repeat(auto-fit,minmax(min(100%,14rem),1fr)); gap: .5rem; max-block-size: 18rem; overflow-y: auto; margin-block-start: .75rem; }
  .products label { display: flex; align-items: center; gap: .5rem; margin: 0; padding: .5rem; border: 1px solid #c5d5dd; border-radius: .4rem; }
  .products label:has(input:checked) { background: #edf4f7; border-color: #20586f; }
  .products input { flex: 0 0 auto; } .required { color: #9f3038; }
</style>
