<script lang="ts">
  import { untrack } from 'svelte';
  import Notice from '$lib/components/Notice.svelte';
  import { money } from '$lib/money';
  let { data, form } = $props();
  let itemId = $state(untrack(() => data.menu.items[0]?.id ?? ''));
  let variantId = $state(untrack(() => data.menu.items[0]?.variants[0]?.id ?? ''));
  let options = $state<string[]>([]);
  let quantity = $state(1);
  const offeringKey = $derived(`${data.offering.merchant_id}/${data.offering.store_id}/${data.offering.publication_generation}`);
  $effect(() => {
    void offeringKey;
    untrack(() => {
      itemId = data.menu.items[0]?.id ?? '';
      variantId = data.menu.items[0]?.variants[0]?.id ?? '';
      options = [];
      quantity = 1;
    });
  });
  let item = $derived(data.menu.items.find((value) => value.id === itemId));
  let selection = $derived({ publication_generation: data.offering.publication_generation, lines: [{ item_id: itemId, variant_id: variantId, quantity, options: options.map((value) => { const [group_id, option_id] = value.split(':'); return { group_id, option_id }; }) }] });
</script>
<h1>{data.offering.display_name}</h1>
<p>Explore the menu and check the price of your selection. No registration is needed.</p>
{#if form?.error}<Notice text={form.error} error />{/if}
<section><h2>Choose your meal</h2>
<form method="POST" action="?/quote">
  <label>Menu item<select bind:value={itemId} onchange={() => { const next = data.menu.items.find((value) => value.id === itemId); variantId = next?.variants[0]?.id ?? ''; options = []; }}>{#each data.menu.items as entry}<option value={entry.id} disabled={!entry.available}>{entry.name}{!entry.available ? ' — unavailable' : ''}</option>{/each}</select></label>
  {#if item}
    <label>Variant<select bind:value={variantId}>{#each item.variants as variant}<option value={variant.id}>{variant.name} · {money(variant.price_minor)}</option>{/each}</select></label>
    {#each item.option_groups as group}
      <fieldset><legend>{group.name} · choose {group.min_selected} to {group.max_selected}</legend>
      {#each group.options as option}<label><input type="checkbox" bind:group={options} value={group.id + ':' + option.id} />{option.name}{option.price_minor ? ' · +' + money(option.price_minor) : ''}</label>{/each}
      </fieldset>
    {/each}
    <label>Quantity<input type="number" min="1" max="99" bind:value={quantity} required /></label>
    <input type="hidden" name="selection" value={JSON.stringify(selection)} />
    <button disabled={!item.available}>Check selection price</button>
  {/if}
</form></section>
{#if form?.quote}
<section aria-label="Selection quote"><h2>Your selection</h2>
{#each form.quote.lines as line}<p>{line.selection.quantity} × {line.item_name} ({line.variant_name}) · {money(line.total_minor)}</p>{/each}
<p><strong>Total {money(form.quote.total_minor)}</strong></p>
<p class="muted">Price quote valid until {new Date(form.quote.expires_at).toLocaleTimeString('de-DE', { timeZone: 'Europe/Berlin' })} (Berlin time). This is not an order or a reservation. Ordering and payment follow in the next slice.</p></section>
{/if}
