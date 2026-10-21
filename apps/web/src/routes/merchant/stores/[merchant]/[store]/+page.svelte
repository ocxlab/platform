<script lang="ts">
  import Notice from '$lib/components/Notice.svelte';
  import MenuEditor from '$lib/components/MenuEditor.svelte';
  import ActionButton from '$lib/components/ActionButton.svelte';
  import Upload from '@lucide/svelte/icons/upload';
  import EyeOff from '@lucide/svelte/icons/eye-off';
  import Save from '@lucide/svelte/icons/save';
  let { data, form } = $props();
</script>
<h1>{data.store.display_name}</h1>
<p>Save changes as a private draft. Publish only when you are ready for customers to see them.</p>
{#if form?.error}<Notice text={form.error} error />{/if}
{#if form?.success}<Notice text={form.success} />{/if}
<section><h2>Publication</h2>
  <p>Published revision: {data.publication.revision ?? 'None'}. Latest draft: {data.draft?.revision ?? 'None'}.</p>
  {#if data.publication.suspended}<Notice text="The provider has suspended public visibility. You can still edit your draft." error />{/if}
  <p><a href={`/stores/${data.store.merchant_id}/${data.store.id}`}>Open customer storefront</a></p>
  <form method="POST" action="?/publish"><input type="hidden" name="key" value={data.request_key + '-publish'} /><input type="hidden" name="generation" value={data.publication.generation} /><input type="hidden" name="revision" value={data.draft?.revision ?? ''} /><ActionButton icon={Upload} label="Publish saved draft" disabled={!data.draft} /></form>
  <form method="POST" action="?/publish"><input type="hidden" name="key" value={data.request_key + '-withdraw'} /><input type="hidden" name="generation" value={data.publication.generation} /><ActionButton icon={EyeOff} label="Withdraw menu" disabled={!data.publication.revision} /></form>
</section>
<section><h2>Edit menu</h2><p>Prices are in euros. Availability is controlled manually; no stock is reserved.</p>
<form method="POST" action="?/save"><input type="hidden" name="key" value={data.request_key + '-draft'} />
  {#key data.draft?.revision}<MenuEditor initial={data.menu} base={data.draft?.revision ?? 0} />{/key}
  <ActionButton icon={Save} label="Save private draft" />
</form></section>
