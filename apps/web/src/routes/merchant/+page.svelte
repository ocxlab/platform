<script lang="ts">
  import Notice from '$lib/components/Notice.svelte';
  let { data, form } = $props();
</script>
<h1>Merchant workspace</h1>
<p>Prepare your offering privately, then publish a revision for customers.</p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if !data.merchant}
  <section><h2>Connect a development credential</h2><p class="muted">Run task dev:credential. The API verifies permissions when you perform an operation. This is not production sign-in.</p>
  <form method="POST" action="?/connect"><label>Merchant identifier<input name="merchant" required autocomplete="off" /></label><label>Temporary merchant credential<input name="token" type="password" required autocomplete="off" /></label><button>Connect merchant</button></form></section>
{:else}
  <section><h2>Create a store</h2><form method="POST" action="?/create"><input type="hidden" name="key" value={data.request_key} /><label>Store name<input name="name" required maxlength="160" /></label><button>Create store</button></form></section>
  <section><h2>Return to a store</h2><p>Use your retained workspace link, or enter the store identifier below.</p><form action="/merchant/open" method="GET"><label>Store identifier<input name="store" required /></label><button>Open store</button></form></section>
  <form method="POST" action="?/disconnect"><button>Disconnect merchant</button></form>
{/if}
