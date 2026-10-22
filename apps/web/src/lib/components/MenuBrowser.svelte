<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import { onMount, tick } from 'svelte';
  import type { components } from '$lib/api/generated';
  import { money } from '$lib/money';
  import LayoutGrid from '@lucide/svelte/icons/layout-grid';
  import List from '@lucide/svelte/icons/list';
  import ListFilter from '@lucide/svelte/icons/list-filter';
  import Search from '@lucide/svelte/icons/search';
  import X from '@lucide/svelte/icons/x';
  import ChevronUp from '@lucide/svelte/icons/chevron-up';
  import Check from '@lucide/svelte/icons/check';
  import { menuName, menuSearchKey } from '$lib/food-menu';
  const { t, locale } = useI18n();
  type Menu = components['schemas']['FoodMenu'];
  let { menu, merchant, store, selected, onchoose }: {
    menu: Menu; merchant: string; store: string; selected: string;
    onchoose: (id: string) => void;
  } = $props();
  let view = $state<'cards' | 'list' | 'select'>('cards');
  let search = $state('');
  let selectedCategories = $state<string[]>([]);
  let viewPicker = $state<HTMLDetailsElement>();
  let categoryPicker = $state<HTMLDetailsElement>();
  let chipRow = $state<HTMLDivElement>();
  let searchInput = $state<HTMLInputElement>();
  let interactive = $state(false);
  onMount(() => { interactive = true; });
  const sections = $derived([
    ...(menu.categories ?? []).map(category => ({ ...category, name: menuName(category, locale()), items: menu.items.filter(item => item.category_id === category.id) })),
    { id: '_other', name: t('menu-uncategorized'), items: menu.items.filter(item => !item.category_id) },
  ].filter(section => section.items.length));
  const activeCategories = $derived(selectedCategories.filter(id => sections.some(section => section.id === id)));
  const searchKey = $derived(menuSearchKey(search));
  const filtered = $derived(sections.filter(section => !activeCategories.length || activeCategories.includes(section.id))
    .map(section => ({ ...section, items: section.items.filter(item => [item.name, ...Object.values(item.translations ?? {})]
      .some(name => menuSearchKey(name ?? '').includes(searchKey))).map(item => ({ ...item, name: menuName(item, locale()) })) }))
    .filter(section => section.items.length));
  const resultCount = $derived(filtered.reduce((count, section) => count + section.items.length, 0));
  function toggleCategory(id: string) {
    selectedCategories = activeCategories.includes(id) ? activeCategories.filter(value => value !== id) : [...activeCategories, id];
  }
  async function removeCategory(id: string) {
    const index = activeCategories.indexOf(id);
    selectedCategories = activeCategories.filter(value => value !== id);
    await tick();
    const chips = chipRow?.querySelectorAll<HTMLButtonElement>('button');
    (chips?.[Math.min(index, chips.length - 1)] ?? searchInput)?.focus({ preventScroll: true });
  }
  function closePicker(picker?: HTMLDetailsElement) {
    if (!picker?.open) return;
    picker.open = false;
    picker.querySelector('summary')?.focus();
  }
  const controls = [
    { id: 'cards', label: 'menu-cards', icon: LayoutGrid },
    { id: 'list', label: 'menu-list', icon: List },
    { id: 'select', label: 'menu-select', icon: ListFilter },
  ] as const;
  const currentView = $derived(controls.find(control => control.id === view)!);
</script>

<svelte:window onclick={(event) => {
  for (const picker of [viewPicker, categoryPicker]) if (picker?.open && !picker.contains(event.target as Node)) picker.open = false;
}} onkeydown={(event) => {
  if (event.key === 'Escape') {
    for (const picker of [viewPicker, categoryPicker]) if (picker?.open) { closePicker(picker); event.preventDefault(); }
  }
}} />
<section id="menu-browse" class="menu-browser" aria-label={t('menu-browse')}>
  <h2>{t('menu-browse')}</h2>
  <div class="menu-toolbar">
    <div class="search-row">
      <label class="menu-search"><span>{t('menu-search')}</span><span class="search-input"><Search size={18} aria-hidden="true" /><input type="search" disabled={!interactive} bind:this={searchInput} bind:value={search} placeholder={t('menu-search-placeholder')} /></span></label>
      {#if search || activeCategories.length}<button class="reset-search" type="button" aria-label={t('menu-clear-filters')} title={t('menu-clear-filters')} onclick={() => { search = ''; selectedCategories = []; searchInput?.focus({ preventScroll: true }); }}><X size={18} aria-hidden="true" /></button>{/if}
    </div>
    <div class="search-feedback">
      {#if activeCategories.length}<div class="category-chips" role="group" aria-label={t('menu-active-categories')} bind:this={chipRow}>
        {#each activeCategories as id (id)}
          {@const section = sections.find(section => section.id === id)!}
          <button class="category-chip" type="button" aria-label={t('menu-remove-filter', { name: section.name })} onclick={() => removeCategory(id)}><bdi dir="auto">{section.name}</bdi><X size={16} aria-hidden="true" /></button>
        {/each}
      </div>{/if}
      <span class="result-count" role="status">{t('menu-result-count', { count: resultCount })}</span>
    </div>
  </div>
  {#if interactive}
    <div class="floating-menu-tools">
      <details class="view-picker" bind:this={viewPicker}>
        <summary class="view-trigger" aria-label={t('menu-current-view', { name: t(currentView.label) })} title={t('menu-current-view', { name: t(currentView.label) })}><currentView.icon size={20} aria-hidden="true" /></summary>
        <div class="picker-panel view-switch" role="group" aria-label={t('menu-view')}>
          {#each controls as control}
            <button type="button" aria-pressed={view === control.id} onclick={() => { view = control.id; closePicker(viewPicker); }}><control.icon size={18} aria-hidden="true" /><span>{t(control.label)}</span>{#if view === control.id}<Check size={18} aria-hidden="true" />{/if}</button>
          {/each}
        </div>
      </details>
      <details class="category-picker" bind:this={categoryPicker}>
        <summary><ListFilter size={18} aria-hidden="true" /><span>{t('menu-categories')}</span>{#if activeCategories.length}<span class="filter-count">{activeCategories.length}</span>{/if}<ChevronUp size={16} aria-hidden="true" /></summary>
        <div class="picker-panel category-options" role="group" aria-label={t('menu-categories')}>
          <div class="picker-heading"><strong>{t('menu-categories')}</strong><button type="button" class="close-picker" aria-label={t('menu-close-controls')} onclick={() => closePicker(categoryPicker)}><X size={18} aria-hidden="true" /></button></div>
          <p>{t('menu-category-filter-help')}</p>
          {#each sections as section}
            <label class="category-option"><input type="checkbox" checked={activeCategories.includes(section.id)} onchange={() => toggleCategory(section.id)} /><bdi dir="auto">{section.name}</bdi><span class="count">{section.items.length}</span></label>
          {/each}
          <button class="clear-categories" type="button" disabled={!activeCategories.length} onclick={() => selectedCategories = []}>{t('menu-all-categories')}</button>
        </div>
      </details>
    </div>
  {/if}
  {#if view === 'select'}
    <label>{t('storefront-item')}
      <select value={selected} onchange={(event) => { if (event.currentTarget.value) onchoose(event.currentTarget.value); }}>
        <option value="">{t('menu-choose')}</option>
        {#each filtered as section}<optgroup label={section.name}>{#each section.items as item}<option value={item.id} disabled={!item.available}>{item.name}{!item.available ? ' ' + t('storefront-unavailable') : ''}</option>{/each}</optgroup>{/each}
      </select>
    </label>
  {:else}
    {#each filtered as section}
      <div id={`menu-category-${section.id}`} class="menu-category">
        <h3><bdi dir="auto">{section.name}</bdi></h3>
        <div class:card-grid={view === 'cards'} class:menu-list={view === 'list'}>
          {#each section.items as item (item.id)}
            <a class="menu-card" class:selected={selected === item.id} class:sold-out={!item.available} aria-disabled={!item.available ? 'true' : undefined}
              href={`?item=${encodeURIComponent(item.id)}#configure-item`} tabindex={item.available ? undefined : -1} aria-label={t('menu-choose-item', { name: item.name })}
              onclick={(event) => { if (item.available && (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)) return; event.preventDefault(); if (item.available) onchoose(item.id); }}>
              <div class="card-photo">{#if item.image_id}<img src={`/media/public/${merchant}/${store}/${item.image_id}`} alt={item.name} loading="lazy" width="480" height="360" />{:else}<div class="photo-placeholder" aria-hidden="true"><span class="placeholder-shape">◇</span></div>{/if}{#if !item.available}<span class="unavailable-badge">{t('basket-sold-out')}</span>{/if}</div>
              <div class="card-copy"><h4><bdi dir="auto">{item.name}</bdi></h4>
                <span class="price">{item.variants.length > 1 ? t('menu-from', { amount: money(Math.min(...item.variants.map(variant => variant.price_minor)), locale()) }) : money(item.variants[0].price_minor, locale())}</span>
                <span class="card-action">{item.available ? t('menu-customize') : t('storefront-unavailable')}</span>
              </div>
            </a>
          {/each}
        </div>
      </div>
    {/each}
  {/if}
  {#if !filtered.length}<p role="status">{t('menu-no-results')}</p>{/if}
</section>

<style>
  .menu-browser { padding: 0; background: transparent; border: 0; }
  .search-row, .search-input { display: flex; gap: .65rem; }
  .search-row { align-items: end; }
  .search-input { align-items: center; }
  .menu-toolbar { position: sticky; inset-block-start: 0; z-index: 4; background: #f5f7f8; padding-block: .6rem; margin-block: 0 .5rem; border-block-end: 1px solid #ccd8de; }
  h2 { margin: 0 0 1rem; }
  .view-switch { display: grid; gap: .25rem; }
  .view-switch button { display: flex; align-items: center; gap: .65rem; background: white; color: #20586f; border: 1px solid transparent; border-radius: .4rem; padding: .5rem .65rem; margin: 0; text-align: start; }
  .view-switch button span { flex: 1; }
  .view-switch button[aria-pressed=true] { background: #e5eff3; font-weight: 600; }
  .view-switch button:hover { background: #cee1ea; border-color: #8caebf; }
  .view-switch button:active { background: #bdd5e0; }
  .menu-search { flex: 1; min-inline-size: 0; margin: 0; }
  .search-feedback { display: flex; align-items: center; flex-wrap: wrap; gap: .5rem 1rem; margin-block-start: .5rem; }
  .category-chips { display: flex; flex: 1; flex-wrap: wrap; gap: .4rem; }
  .category-chip { display: inline-flex; gap: .5rem; align-items: center; min-block-size: 44px; margin: 0; padding: .35rem .7rem; border-radius: 2rem; background: #e5eff3; border: 1px solid #9ab2be; color: #20586f; text-align: start; }
  .category-chip:hover { background: #cee1ea; border-color: #20586f; }
  .category-chip :global(svg) { flex: none; }
  .result-count { color: #526671; font-size: .85rem; margin-inline-start: auto; white-space: nowrap; }
  .search-input { block-size: 44px; box-sizing: border-box; }
  .view-switch button { min-block-size: 44px; box-sizing: border-box; }
  .search-input { border: 1px solid #9ab2be; border-radius: .6rem; background: white; padding-inline: .85rem; color: #20586f; }
  /* Highlight the composite field, not a second rectangle around its inner input. */
  .search-input:focus-within { border-color: #20586f; box-shadow: 0 0 0 1px #20586f; }
  .search-input input { border: 0; background: transparent; margin: 0; padding-inline: 0; min-inline-size: 0; }
  .search-input input:focus-visible { outline: none; }
  .floating-menu-tools { position: fixed; inset-inline-start: max(1.4rem, (100vw - 1080px) / 2 + 1.4rem); bottom: max(1rem, env(safe-area-inset-bottom)); display: flex; gap: .5rem; z-index: 6; max-inline-size: calc(100vw - 2.8rem); }
  details { position: relative; min-inline-size: 0; }
  summary { display: flex; gap: .4rem; align-items: center; cursor: pointer; height: 44px; padding: .5rem .7rem; border: 1px solid #9ab2be; border-radius: .6rem; color: #20586f; background: white; list-style: none; box-shadow: 0 3px 12px #102c3b22; }
  summary::-webkit-details-marker { display: none; }
  summary.view-trigger { inline-size: 44px; padding: 0; justify-content: center; }
  summary:hover { background: #e5eff3; border-color: #20586f; }
  summary > span:not(.filter-count) { min-inline-size: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  summary :global(svg) { flex: none; }
  details[open] summary { border-color: #20586f; background: #e5eff3; }
  .filter-count { border-radius: 1rem; background: #20586f; color: white; padding-inline: .4rem; font-size: .85rem; }
  .picker-panel { position: absolute; bottom: calc(100% + .5rem); inset-inline-start: 0; inline-size: 12rem; max-inline-size: calc(100vw - 2.8rem); max-block-size: 60dvh; overflow-y: auto; padding: .65rem; background: white; border: 1px solid #9ab2be; border-radius: .75rem; box-shadow: 0 5px 20px #102c3b33; overscroll-behavior: contain; }
  .category-options { inline-size: 20rem; }
  .picker-heading { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }
  .picker-panel p { font-size: .85rem; color: #526671; margin-block: .3rem .6rem; }
  .picker-panel button { margin: 0; }
  .close-picker { display: grid; place-items: center; background: white; color: #20586f; border: 0; min-inline-size: 44px; min-block-size: 44px; padding: .4rem; }
  .category-option { display: flex; align-items: center; gap: .6rem; min-block-size: 44px; margin: 0; padding: .35rem; font-weight: 400; cursor: pointer; border-radius: .4rem; }
  .category-option:has(:checked) { background: #e5eff3; }
  .category-option:hover { background: #cee1ea; }
  .category-option input { flex: none; inline-size: 18px; block-size: 18px; margin: 0; accent-color: #20586f; }
  .category-option bdi { flex: 1; }
  .clear-categories, .reset-search { background: white; color: #20586f; min-block-size: 44px; }
  .reset-search { display: grid; place-items: center; inline-size: 44px; block-size: 44px; flex: none; padding: 0; margin: 0; }
  .reset-search:hover, .clear-categories:not(:disabled):hover { background: #e5eff3; border-color: #20586f; }
  .clear-categories { inline-size: 100%; margin-block-start: .5rem !important; }
  .count { font-size: .8rem; color: #526671; }
  .menu-category { scroll-margin-block-start: 7rem; margin-block-end: 2rem; }
  h3 { font-size: 1.25rem; margin-block: 1rem; }
  .card-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 14rem), 1fr)); gap: 1rem; }
  .menu-card { display: flex; flex-direction: column; border: 1px solid #c5d5dd; border-radius: .85rem; overflow: hidden; text-decoration: none; color: #193441; background: white; min-inline-size: 0; }
  .menu-card[href]:hover, .menu-card.selected { border-color: #20586f; box-shadow: 0 3px 12px #183f5018; }
  .menu-card:focus-visible { outline: 3px solid #20586f; outline-offset: 3px; }
  .menu-card img, .photo-placeholder { inline-size: 100%; aspect-ratio: 4/3; object-fit: cover; block-size: auto; }
  .photo-placeholder { display: grid; place-items: center; background: #e6eff2; color: #70909e; }
  .placeholder-shape { font-size: 3rem; }
  .card-copy { display: flex; flex: 1; flex-direction: column; align-items: start; padding: 1rem; gap: .5rem; }
  h4 { font-size: 1.05rem; margin: 0; overflow-wrap: anywhere; }
  .price { font-weight: 600; }
  .card-action { color: #20586f; font-size: .85rem; margin-block-start: auto; }
  .sold-out { color: #526671; }
  .sold-out { border-color: #98a6ad; cursor: not-allowed; }
  .sold-out img { filter: grayscale(1); opacity: .4; }
  .card-photo { position: relative; }
  .card-photo img { display: block; }
  .unavailable-badge { position: absolute; inset: 0; margin: auto; inline-size: max-content; block-size: max-content; max-inline-size: calc(100% - 1rem); padding: .4rem .75rem; border-radius: .4rem; color: white; background: #672b32; font-size: .9rem; font-weight: 700; }
  .menu-list { display: grid; gap: .65rem; }
  .menu-list .menu-card { flex-direction: row-reverse; align-items: stretch; }
  .menu-list .card-photo, .menu-list img, .menu-list .photo-placeholder { inline-size: 7.5rem; block-size: 7.5rem; flex: 0 0 7.5rem; }
  .menu-list .card-copy { gap: .3rem; padding: .75rem 1rem; }
  @media (max-width: 760px) { .floating-menu-tools { bottom: calc(max(1rem, env(safe-area-inset-bottom)) + 3.25rem); } .category-options { position: fixed; inset-inline-start: 1.4rem; bottom: calc(max(1rem, env(safe-area-inset-bottom)) + 3.25rem + 44px + .5rem); inline-size: min(20rem, calc(100vw - 2.8rem)); } .menu-browser { padding-block-end: 3.25rem; } }
  @media (max-width: 480px) { .view-switch button { padding: .6rem; } .card-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .65rem; } .card-copy { padding: .75rem; } }
</style>
