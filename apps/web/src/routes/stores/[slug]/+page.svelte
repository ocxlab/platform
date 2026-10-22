<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t, locale } = useI18n();
  import { tick, untrack } from 'svelte';
  import MenuBrowser from '$lib/components/MenuBrowser.svelte';
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Calculator from '@lucide/svelte/icons/calculator';
  import ShoppingBag from '@lucide/svelte/icons/shopping-bag';
  import { money } from '$lib/money';
  let { data, form } = $props();
  let itemId = $state(untrack(() => data.selectedItem));
  let configurationHeading = $state<HTMLHeadingElement>();
  async function chooseItem(id: string) {
    itemId = id;
    await tick();
    configurationHeading?.focus();
    configurationHeading?.scrollIntoView({ block: 'start' });
  }
  const offeringKey = $derived(`${data.offering.merchant_id}/${data.offering.store_id}/${data.offering.publication_generation}`);
  let previousOffering = untrack(() => offeringKey);
  $effect(() => {
    if (offeringKey === previousOffering) return;
    previousOffering = offeringKey;
    untrack(() => {
      itemId = data.selectedItem;
    });
  });
  let item = $derived(data.menu.items.find((value) => value.id === itemId));
</script>
<h1><bdi dir="auto">{data.offering.display_name}</bdi></h1>
<p>{t('storefront-intro')}</p>
{#if data.offering.store_id === '99598cf5-eded-4394-a053-751a1cb20db2'}<p class="muted">{t('media-demo')}</p>{/if}
{#if !data.ordering.accepting_orders}<Notice text={t('ordering-paused-customer')} />{/if}
{#if form?.error}<Notice text={form.error} error />{/if}
<MenuBrowser menu={data.menu} merchant={data.offering.merchant_id} store={data.offering.store_id} selected={itemId} onchoose={chooseItem} />
{#if item}
<section id="configure-item" class="configuration"><h2 tabindex="-1" bind:this={configurationHeading}>{t('menu-configure', { name: item.name })}</h2>
<a href="#menu-browse">{t('menu-back')}</a>
<p class="muted">{t('menu-customize-help')}</p>
<form method="POST" action={`?/quote&item=${encodeURIComponent(itemId)}`}>
  <input type="hidden" name="publication_generation" value={data.offering.publication_generation} />
  <input type="hidden" name="item_id" value={itemId} />
  {#if item}
    {#if item.image_id}<img class="dish-photo" src={`/media/public/${data.offering.merchant_id}/${data.offering.store_id}/${item.image_id}`} alt={item.name} width="480" height="360" />{/if}
    {#key `${offeringKey}/${itemId}`}
    <label>{t('storefront-variant')}<select name="variant_id" dir="auto">{#each item.variants as variant}<option dir="auto" value={variant.id}>{variant.name} · {money(variant.price_minor, locale())}</option>{/each}</select></label>
    {#each item.option_groups as group}
      <fieldset><legend><bdi dir="auto">{group.name}</bdi> · {t('storefront-options', { min: group.min_selected, max: group.max_selected })}</legend>
      {#each group.options as option}<label><input type="checkbox" name="option" value={group.id + ':' + option.id} /><bdi dir="auto">{option.name}</bdi> · {option.price_minor ? '+' : ''}<bdi>{money(option.price_minor, locale())}</bdi></label>{/each}
      </fieldset>
    {/each}
    <label>{t('storefront-quantity')}<input name="quantity" type="number" min="1" max="99" value="1" required /></label>
    <ActionButton icon={Calculator} label={t('storefront-check')} disabled={!item.available || !data.ordering.accepting_orders} />
    {/key}
  {/if}
</form></section>
{/if}
{#if data.quote}
<section id="selection-quote" aria-label={t('storefront-quote')}><h2>{t('storefront-selection')}</h2>
{#each data.quote.lines as line}<p>{line.selection.quantity} × <bdi dir="auto">{line.item_name}</bdi> (<bdi dir="auto">{line.variant_name}</bdi>) · <bdi>{money(line.total_minor, locale())}</bdi>{#each line.selected_options ?? [] as option}<br /><small><bdi dir="auto">{option.group_name}</bdi>: <bdi dir="auto">{option.option_name}</bdi></small>{/each}</p>{/each}
<p><strong>{t('storefront-total', { amount: money(data.quote.total_minor, locale()) })}</strong></p>
<p>{t('storefront-pickup-help')}</p>
<p class="muted">{t('storefront-quote-help', { time: new Date(data.quote.expires_at).toLocaleTimeString(locale(), { timeZone: 'Europe/Berlin' }) })}</p>
</section>
{/if}
<style>
  .dish-photo { display: block; inline-size: min(100%, 30rem); block-size: auto; aspect-ratio: 4/3; object-fit: cover; border-radius: .75rem; margin-block: 1rem; }
  .configuration { scroll-margin-block-start: 1rem; }
  .configuration h2:focus { outline: 0; }
</style>
{#if data.intent}
<form method="POST" action="?/place">
  <input type="hidden" name="quote" value={data.intent.quote} />
  <input type="hidden" name="key" value={data.intent.key} />
  <ActionButton icon={ShoppingBag} label={data.quote ? t('storefront-place') : t('storefront-recover')} />
</form>
{/if}
