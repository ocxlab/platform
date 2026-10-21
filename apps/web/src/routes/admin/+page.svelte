<script lang="ts">
  import Notice from '$lib/components/Notice.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import LogIn from '@lucide/svelte/icons/log-in';
  import LogOut from '@lucide/svelte/icons/log-out';
  import Search from '@lucide/svelte/icons/search';
  import Eye from '@lucide/svelte/icons/eye';
  import EyeOff from '@lucide/svelte/icons/eye-off';
  let { data, form } = $props();
</script>
<h1>Provider administration</h1>
<p>Inspect a store's publication and manage public visibility without editing the merchant's menu.</p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if form?.success}<Notice text={form.success} />{/if}
{#if !data.connected}
<section><h2>Connect a development provider credential</h2><p class="muted">Run task dev:admin-credential. Merchant credentials are not accepted. Permissions are verified by the API when an operation is performed.</p><form method="POST" action="?/connect"><label>Temporary provider credential<input type="password" name="token" required autocomplete="off" /></label><ActionButton icon={LogIn} label="Connect provider" /></form></section>
{:else}
<section><h2>Find publication</h2><form method="POST" action="?/inspect"><label>Merchant identifier<input name="merchant" required value={form?.publication?.merchant_id ?? ''} /></label><label>Store identifier<input name="store" required value={form?.publication?.store_id ?? ''} /></label><ActionButton icon={Search} label="Inspect publication" /></form></section>
{#if form?.publication}
<section><h2>Publication visibility</h2><p>Generation {form.publication.generation}. Chosen revision: {form.publication.revision ?? 'None'}. Visibility: {form.publication.suspended ? 'Suspended' : 'Not suspended'}.</p>
<form method="POST" action="?/suspend"><input type="hidden" name="merchant" value={form.publication.merchant_id} /><input type="hidden" name="store" value={form.publication.store_id} /><input type="hidden" name="generation" value={form.publication.generation} /><input type="hidden" name="key" value={data.request_key} /><input type="hidden" name="suspended" value={String(!form.publication.suspended)} /><label>Reason for this action<textarea name="reason" required maxlength="500"></textarea></label><ActionButton icon={form.publication.suspended ? Eye : EyeOff} label={form.publication.suspended ? 'Restore visibility' : 'Suspend visibility'} /></form>
</section>
{/if}
<form method="POST" action="?/disconnect"><ActionButton icon={LogOut} label="Disconnect provider" /></form>
{/if}
