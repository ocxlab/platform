<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import { onMount } from 'svelte';
  import type { components } from '$lib/api/generated';
  import { money } from '$lib/money';
  import LayoutGrid from '@lucide/svelte/icons/layout-grid';
  import List from '@lucide/svelte/icons/list';
  import ListFilter from '@lucide/svelte/icons/list-filter';
  import Search from '@lucide/svelte/icons/search';
  import { menuName } from '$lib/food-menu';
  const { t, locale } = useI18n();
  type Menu = components['schemas']['FoodMenu'];
  let { menu, merchant, store, selected, onchoose }: {
    menu: Menu; merchant: string; store: string; selected: string;
    onchoose: (id: string) => void;
  } = $props();
  let view = $state<'cards' | 'list' | 'select'>('cards');
  let search = $state('');
  let interactive = $state(false);
  onMount(() => { interactive = true; });
  const sections = $derived([
    ...(menu.categories ?? []).map(category => ({ ...category, name: menuName(category, locale()), items: menu.items.filter(item => item.category_id === category.id) })),
    { id: '_other', name: t('menu-uncategorized'), items: menu.items.filter(item => !item.category_id) },
  ].filter(section => section.items.length));
  const filtered = $derived(sections.map(section => ({ ...section, items: section.items.map(item => ({ ...item, name: menuName(item, locale()) })).filter(item => item.name.toLocaleLowerCase(locale()).includes(search.trim().toLocaleLowerCase(locale()))) })).filter(section => section.items.length));
  const controls = [
    { id: 'cards', label: 'menu-cards', icon: LayoutGrid },
    { id: 'list', label: 'menu-list', icon: List },
    { id: 'select', label: 'menu-select', icon: ListFilter },
  ] as const;
</script>

<section id="menu-browse" class="menu-browser" aria-label={t('menu-browse')}>
  <h2>{t('menu-browse')}</h2>
  <div class="menu-toolbar">
    <label class="menu-search"><span>{t('menu-search')}</span><span class="search-input"><Search size={18} aria-hidden="true" /><input type="search" disabled={!interactive} bind:value={search} placeholder={t('menu-search-placeholder')} /></span></label>
    <div class="view-switch" role="group" aria-label={t('menu-view')}>
      {#each controls as control}
        <button type="button" disabled={!interactive} aria-pressed={view === control.id} onclick={() => view = control.id}><control.icon size={18} aria-hidden="true" />{t(control.label)}</button>
      {/each}
    </div>
  </div>
  {#if view !== 'select'}<nav class="category-nav" aria-label={t('menu-categories')}>
    {#each filtered as section}<a href={`#menu-category-${section.id}`}><bdi dir="auto">{section.name}</bdi><span class="count">{section.items.length}</span></a>{/each}
  </nav>{/if}
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
  .menu-toolbar, .view-switch, .category-nav, .search-input { display: flex; align-items: center; gap: .65rem; }
  .menu-toolbar { justify-content: space-between; align-items: end; flex-wrap: wrap; margin-block: 1rem .5rem; }
  h2 { margin: 0 0 1rem; }
  .view-switch { gap: .25rem; flex-wrap: wrap; }
  .view-switch button { display: inline-flex; align-items: center; gap: .4rem; background: white; color: #20586f; border: 1px solid #b8ccd5; border-radius: .5rem; padding: .65rem .8rem; margin: 0; }
  .view-switch button[aria-pressed=true] { background: #20586f; color: white; border-color: #20586f; }
  .menu-search { flex: 1 1 18rem; max-inline-size: 32rem; margin: 0; }
  .search-input { min-block-size: 44px; box-sizing: border-box; }
  .view-switch button { min-block-size: 44px; box-sizing: border-box; }
  .search-input { border: 1px solid #9ab2be; border-radius: .6rem; background: white; padding-inline: .85rem; color: #20586f; }
  .search-input input { border: 0; background: transparent; margin: 0; padding-inline: 0; min-inline-size: 0; }
  .category-nav { flex-wrap: wrap; padding-block: .75rem 1rem; margin-block-end: .5rem; }
  .category-nav a { flex: 0 0 auto; display: inline-flex; align-items: center; gap: .5rem; border: 1px solid #b8ccd5; border-radius: 2rem; padding: .55rem .9rem; text-decoration: none; background: white; }
  .count { font-size: .8rem; color: #526671; }
  .menu-category { scroll-margin-block-start: 1rem; margin-block-end: 2rem; }
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
  @media (max-width: 480px) { .view-switch button { padding: .6rem; } .card-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .65rem; } .card-copy { padding: .75rem; } }
</style>
