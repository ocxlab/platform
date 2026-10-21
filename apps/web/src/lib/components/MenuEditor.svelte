<script lang="ts">
  import { untrack } from 'svelte';
  import type { components } from '$lib/api/generated';
  type Menu = components['schemas']['FoodMenu'];
  let { initial, base }: { initial: Menu; base: number } = $props();
  let menu = $state<Menu>(untrack(() => structuredClone(initial)));
  const identifier = () => Array.from(crypto.getRandomValues(new Uint8Array(12)), (b) => b.toString(16).padStart(2, '0')).join('');
  const variant = () => ({ id: identifier(), name: 'Regular', price_minor: 0 });
  function addItem() { menu.items.push({ id: identifier(), name: 'New item', available: true, variants: [variant()], option_groups: [] }); }
  function removeItem(id: string) {
    menu.items = menu.items.filter((item) => item.id !== id);
  }
</script>
<input type="hidden" name="document" value={JSON.stringify(menu)} />
<input type="hidden" name="base" value={base} />
<p class="muted">Removing an item changes this draft only. Save and publish the draft to update the customer menu.</p>
{#each menu.items as item (item.id)}
  <fieldset><legend>Menu item</legend>
    <label>Item name<input bind:value={item.name} required maxlength="160" /></label>
    <label><input type="checkbox" bind:checked={item.available} />Available to order</label>
    {#each item.variants as choice (choice.id)}
      <div class="row"><label>Variant name<input bind:value={choice.name} required maxlength="80" /></label><label>Variant price in EUR<input type="number" min="0" step="0.01" max="10000000" required value={choice.price_minor / 100} oninput={(e) => choice.price_minor = Math.round(e.currentTarget.valueAsNumber * 100)} /></label></div>
    {/each}
    <button type="button" onclick={() => item.variants.push(variant())} disabled={item.variants.length >= 30}>Add variant</button>
    {#each item.option_groups as group (group.id)}
      <fieldset><legend>Extras or required choices</legend>
        <label>Choice group name<input bind:value={group.name} required maxlength="80" /></label>
        <div class="row"><label>Minimum choices<input type="number" min="0" max="30" bind:value={group.min_selected} required /></label><label>Maximum choices<input type="number" min="1" max="30" bind:value={group.max_selected} required /></label></div>
        {#each group.options as option (option.id)}
          <div class="row"><label>Option name<input bind:value={option.name} required maxlength="80" /></label><label>Extra price in EUR<input type="number" min="0" step="0.01" max="10000000" required value={option.price_minor / 100} oninput={(e) => option.price_minor = Math.round(e.currentTarget.valueAsNumber * 100)} /></label></div>
        {/each}
        <button type="button" onclick={() => group.options.push({ id: identifier(), name: 'New option', price_minor: 0 })} disabled={group.options.length >= 30}>Add option</button>
      </fieldset>
    {/each}
    <button type="button" onclick={() => item.option_groups.push({ id: identifier(), name: 'Choose an extra', min_selected: 0, max_selected: 1, options: [{ id: identifier(), name: 'Extra', price_minor: 0 }] })} disabled={item.option_groups.length >= 20}>Add choice group</button>
    <button type="button" aria-label={`Remove menu item ${item.name}`} onclick={() => removeItem(item.id)}>Remove item</button>
  </fieldset>
{/each}
{#if menu.items.length === 0}
  <p>Add at least one item before saving. To hide an existing published menu instead, use Withdraw menu.</p>
{/if}
<button type="button" onclick={addItem} disabled={menu.items.length >= 200}>Add menu item</button>
