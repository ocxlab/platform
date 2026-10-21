<script lang="ts">
  import Notice from '$lib/components/Notice.svelte';
  let { data, form } = $props();
</script>
<h1>Provider administration</h1>
<p>Inspect a store's publication and manage public visibility without editing the merchant's menu.</p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if form?.success}<Notice text={form.success} />{/if}
{#if !data.connected}
<section><h2>Connect a development provider credential</h2><p class="muted">Run task dev:admin-credential. Merchant credentials are not accepted. Permissions are verified by the API when an operation is performed.</p><form method="POST" action="?/connect"><label>Temporary provider credential<input type="password" name="token" required autocomplete="off" /></label><button>Connect provider</button></form></section>
{:else}
<section><h2>Find publication</h2><form method="POST" action="?/inspect"><label>Merchant identifier<input name="merchant" required value={form?.publication?.merchant_id ?? ''} /></label><label>Store identifier<input name="store" required value={form?.publication?.store_id ?? ''} /></label><button>Inspect publication</button></form></section>
{#if form?.publication}
<section><h2>Publication visibility</h2><p>Generation {form.publication.generation}. Chosen revision: {form.publication.revision ?? 'None'}. Visibility: {form.publication.suspended ? 'Suspended' : 'Not suspended'}.</p>
<form method="POST" action="?/suspend"><input type="hidden" name="merchant" value={form.publication.merchant_id} /><input type="hidden" name="store" value={form.publication.store_id} /><input type="hidden" name="generation" value={form.publication.generation} /><input type="hidden" name="key" value={data.request_key} /><input type="hidden" name="suspended" value={String(!form.publication.suspended)} /><label>Reason for this action<textarea name="reason" required maxlength="500"></textarea></label><button>{form.publication.suspended ? 'Restore visibility' : 'Suspend visibility'}</button></form>
</section>
{/if}
<form method="POST" action="?/disconnect"><button>Disconnect provider</button></form>
{/if}
