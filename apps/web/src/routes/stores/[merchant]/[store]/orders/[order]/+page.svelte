<script lang="ts">
  import { money } from '$lib/money';
  let { data } = $props();
</script>
<h1>Pickup order placed</h1>
<p>Awaiting restaurant acceptance. This development order has not been charged or sent to the kitchen.</p>
<section aria-label="Order details">
  <h2>Your agreed selection</h2>
  {#each data.order.terms.lines as line}
    <p>{line.selection.quantity} × {line.item_name} ({line.variant_name}) · {money(line.total_minor)}
    {#each line.selected_options ?? [] as option}<br /><small>{option.group_name}: {option.option_name}</small>{/each}</p>
  {/each}
  <p><strong>Total {money(data.order.terms.total_minor)}</strong></p>
  <p>Fulfilment: pickup · Payment: not started · Preparation: not started</p>
  <p class="muted">Order reference {data.order.id}</p>
</section>
<p>You can return to this page in this browser for seven days from the start of guest checkout. Keep the checkout cookie; deleting it removes guest access. No account was created.</p>
<a href={`/stores/${data.order.merchant_id}/${data.order.store_id}`}>Back to the menu</a>
