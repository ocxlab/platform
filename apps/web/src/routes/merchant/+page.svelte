<script lang="ts">
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import LogIn from '@lucide/svelte/icons/log-in';
  import LogOut from '@lucide/svelte/icons/log-out';
  import Plus from '@lucide/svelte/icons/plus';
  let { data, form } = $props();
  const dateFormat = new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Europe/Berlin' });
</script>
<h1>Merchant workspace</h1>
<p>Prepare your offering privately, then publish a revision for customers.</p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if data.stores_error}<Notice text={data.stores_error} error />{/if}
{#if !data.merchant}
  <section><h2>Connect a development credential</h2><p class="muted">Run task dev:credential. The API verifies permissions when you perform an operation. This is not production sign-in.</p>
  <form method="POST" action="?/connect"><label>Merchant identifier<input name="merchant" required autocomplete="off" /></label><label>Temporary merchant credential<input name="token" type="password" required autocomplete="off" /></label><ActionButton icon={LogIn} label="Connect merchant" /></form></section>
{:else}
  <section><h2>Create a store</h2><form method="POST" action="?/create"><input type="hidden" name="key" value={data.request_key} /><label>Store name<input name="name" required maxlength="160" /></label><ActionButton icon={Plus} label="Create store" /></form></section>
  <section aria-labelledby="managed-stores"><h2 id="managed-stores">Your stores</h2>
    {#if data.stores.length}
      <!-- svelte-ignore a11y_no_noninteractive_tabindex (Keyboard users need to scroll the table on narrow screens.) -->
      <div class="store-table" role="region" aria-label="Managed stores table" tabindex="0">
        <table>
          <caption>Latest saved drafts, including unpublished changes. Updated times use Europe/Berlin and fall back to store creation before the first save.</caption>
          <thead><tr><th scope="col">Store name</th><th scope="col">Menu items</th><th scope="col">Draft revision</th><th scope="col">Last updated</th></tr></thead>
          <tbody>{#each data.stores as store (store.id)}
            <tr>
              <th scope="row"><a href={`/merchant/stores/${data.merchant}/${store.id}`}>{store.display_name}</a></th>
              <td>{store.menu_item_count ?? '—'}</td>
              <td>{store.draft_revision || 'No draft'}</td>
              <td><time datetime={store.updated_at}>{dateFormat.format(new Date(store.updated_at))}</time></td>
            </tr>
          {/each}</tbody>
        </table>
      </div>
    {:else if !data.stores_error}
      <p>{data.after ? 'There are no more stores on this page.' : 'You have no stores yet. Create your first store above.'}</p>
    {/if}
    <nav aria-label="Store pages">
      {#if data.after}<a href="/merchant">First page</a>{/if}
      {#if data.next_after}<a href={`/merchant?after=${data.next_after}`}>Next stores</a>{/if}
    </nav>
  </section>
  <form method="POST" action="?/disconnect"><ActionButton icon={LogOut} label="Disconnect merchant" /></form>
{/if}
<style>
  .store-table { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; text-align: left; }
  caption { text-align: left; color: #52636d; font-size: .9rem; padding-bottom: .75rem; }
  th, td { padding: .75rem; border-bottom: 1px solid #ccd8de; }
  thead { background: #f1f6f8; color: #20586f; }
  tbody th { font-weight: 500; }
  td:last-child { white-space: nowrap; }
</style>
