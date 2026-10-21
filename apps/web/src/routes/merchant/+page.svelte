<script lang="ts">
  import Notice from '$lib/components/Notice.svelte';
  let { data, form } = $props();
</script>
<h1>Merchant workspace</h1>
<p>Prepare your offering privately, then publish a revision for customers.</p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if data.stores_error}<Notice text={data.stores_error} error />{/if}
{#if !data.merchant}
  <section><h2>Connect a development credential</h2><p class="muted">Run task dev:credential. The API verifies permissions when you perform an operation. This is not production sign-in.</p>
  <form method="POST" action="?/connect"><label>Merchant identifier<input name="merchant" required autocomplete="off" /></label><label>Temporary merchant credential<input name="token" type="password" required autocomplete="off" /></label><button>Connect merchant</button></form></section>
{:else}
  <section><h2>Create a store</h2><form method="POST" action="?/create"><input type="hidden" name="key" value={data.request_key} /><label>Store name<input name="name" required maxlength="160" /></label><button>Create store</button></form></section>
  <section aria-labelledby="managed-stores"><h2 id="managed-stores">Your stores</h2>
    {#if data.stores.length}
      <ul>{#each data.stores as store (store.id)}
        <li><a href={`/merchant/stores/${data.merchant}/${store.id}`}>{store.display_name}</a></li>
      {/each}</ul>
    {:else if !data.stores_error}
      <p>{data.after ? 'There are no more stores on this page.' : 'You have no stores yet. Create your first store above.'}</p>
    {/if}
    <nav aria-label="Store pages">
      {#if data.after}<a href="/merchant">First page</a>{/if}
      {#if data.next_after}<a href={`/merchant?after=${data.next_after}`}>Next stores</a>{/if}
    </nav>
  </section>
  <form method="POST" action="?/disconnect"><button>Disconnect merchant</button></form>
{/if}
