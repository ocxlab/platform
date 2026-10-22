<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  const { t, locale } = useI18n();
  import { untrack } from 'svelte';
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Calculator from '@lucide/svelte/icons/calculator';
  import ShoppingBag from '@lucide/svelte/icons/shopping-bag';
  import { money } from '$lib/money';
  let { data, form } = $props();
  let itemId = $state(untrack(() => data.menu.items[0]?.id ?? ''));
  const offeringKey = $derived(`${data.offering.merchant_id}/${data.offering.store_id}/${data.offering.publication_generation}`);
  let previousOffering = untrack(() => offeringKey);
  $effect(() => {
    if (offeringKey === previousOffering) return;
    previousOffering = offeringKey;
    untrack(() => {
      itemId = data.menu.items[0]?.id ?? '';
    });
  });
  let item = $derived(data.menu.items.find((value) => value.id === itemId));
</script>
<h1><bdi dir="auto">{data.offering.display_name}</bdi></h1>
<p>{t('storefront-intro')}</p>
{#if !data.ordering.accepting_orders}<Notice text={t('ordering-paused-customer')} />{/if}
{#if form?.error}<Notice text={form.error} error />{/if}
<section><h2>{t('storefront-choose')}</h2>
<form method="POST" action="?/quote">
  <input type="hidden" name="publication_generation" value={data.offering.publication_generation} />
  <label>{t('storefront-item')}<select name="item_id" dir="auto" bind:value={itemId}>{#each data.menu.items as entry}<option dir="auto" value={entry.id} disabled={!entry.available}>{entry.name}{!entry.available ? ' ' + t('storefront-unavailable') : ''}</option>{/each}</select></label>
  {#if item}
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
{#if data.quote}
<section aria-label={t('storefront-quote')}><h2>{t('storefront-selection')}</h2>
{#each data.quote.lines as line}<p>{line.selection.quantity} × <bdi dir="auto">{line.item_name}</bdi> (<bdi dir="auto">{line.variant_name}</bdi>) · <bdi>{money(line.total_minor, locale())}</bdi>{#each line.selected_options ?? [] as option}<br /><small><bdi dir="auto">{option.group_name}</bdi>: <bdi dir="auto">{option.option_name}</bdi></small>{/each}</p>{/each}
<p><strong>{t('storefront-total', { amount: money(data.quote.total_minor, locale()) })}</strong></p>
<p>{t('storefront-pickup-help')}</p>
<p class="muted">{t('storefront-quote-help', { time: new Date(data.quote.expires_at).toLocaleTimeString(locale(), { timeZone: 'Europe/Berlin' }) })}</p>
</section>
{/if}
{#if data.intent}
<form method="POST" action="?/place">
  <input type="hidden" name="quote" value={data.intent.quote} />
  <input type="hidden" name="key" value={data.intent.key} />
  <ActionButton icon={ShoppingBag} label={data.quote ? t('storefront-place') : t('storefront-recover')} />
</form>
{/if}
