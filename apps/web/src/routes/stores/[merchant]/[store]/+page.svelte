<script lang="ts">
  import { untrack } from 'svelte';
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Calculator from '@lucide/svelte/icons/calculator';
  import ShoppingBag from '@lucide/svelte/icons/shopping-bag';
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
    <ActionButton icon={Calculator} label="Check selection price" disabled={!item.available} />
  {/if}
</form></section>
{#if data.quote}
<section aria-label="Selection quote"><h2>Your selection</h2>
{#each data.quote.lines as line}<p>{line.selection.quantity} × {line.item_name} ({line.variant_name}) · {money(line.total_minor)}{#each line.selected_options ?? [] as option}<br /><small>{option.group_name}: {option.option_name}</small>{/each}</p>{/each}
<p><strong>Total {money(data.quote.total_minor)}</strong></p>
<p>Pickup · awaiting restaurant acceptance after placement. Payment and kitchen handling are not connected yet.</p>
<p class="muted">Price quote valid until {new Date(data.quote.expires_at).toLocaleTimeString('de-DE', { timeZone: 'Europe/Berlin' })} (Berlin time). This is not an order or a reservation. Placing it agrees to the selections and total shown above.</p>
</section>
{/if}
{#if data.intent}
<form method="POST" action="?/place">
  <input type="hidden" name="quote" value={data.intent.quote} />
  <input type="hidden" name="key" value={data.intent.key} />
  <ActionButton icon={ShoppingBag} label={data.quote ? 'Place pickup order' : 'Recover placement result'} />
</form>
{/if}
