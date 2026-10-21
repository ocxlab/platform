<script lang="ts">
  import { provideLocale } from '$lib/i18n/context';
  import { translate } from '$lib/i18n/messages';
  import { direction } from '$lib/i18n/locales';
  import LanguagePicker from '$lib/components/LanguagePicker.svelte';
  let { children, data } = $props();
  provideLocale(() => data.locale);
  const t = (key: string) => translate(key, data.locale);
  $effect(() => { document.documentElement.lang = data.locale; document.documentElement.dir = direction(data.locale); });
</script>
<svelte:head><title>OCP Lab</title><meta name="description" content={t('ui-home-title')} /></svelte:head>
<header><a href="/" class="brand" dir="ltr">OCP Lab</a><nav aria-label={t('ui-main')}><a href="/merchant">{t('ui-merchant')}</a><a href="/admin">{t('ui-admin')}</a></nav><LanguagePicker /></header>
<main>{@render children()}</main>
<footer>{t('ui-footer')}</footer>
<style>
  :global(*) { box-sizing: border-box; }
  :global(body) { margin: 0; background: #f5f7f8; color: #1a2b35; font-family: system-ui, sans-serif; line-height: 1.55; }
  header, main, footer { max-width: 1080px; margin: auto; padding: 1.4rem; }
  header { display:flex; flex-wrap:wrap; align-items:center; justify-content:space-between; border-bottom: 1px solid #ccd8de; gap:1rem; }
  nav { display:flex; gap:1rem; flex-wrap:wrap; } .brand { font-weight:750; font-size:1.4rem; text-decoration:none; }
  :global(a) { color: #20586f; } :global(h1), :global(h2), :global(h3) { color:#20586f; line-height:1.2; }
  :global(section), :global(fieldset), :global(article) { background:white; border:1px solid #ccd8de; border-radius:.5rem; padding:1.2rem; margin:1rem 0; min-width:0; }
  :global(label) { display:block; margin:.8rem 0; font-weight:600; } :global(label input), :global(label textarea), :global(label select) { display:block; width:100%; margin-top:.3rem; }
  :global(input), :global(textarea), :global(select), :global(button) { font:inherit; padding:.55rem .7rem; border:1px solid #708692; border-radius:.3rem; }
  :global(button) { background:#256078; color:white; cursor:pointer; margin-block:.4rem; margin-inline:0 .4rem; } :global(button:disabled) { opacity:.6; cursor:default; }
  :global(input[type=checkbox]) { width:auto; display:inline-block; margin-inline-end:.5rem; }
  :global(input:not([type=hidden])), :global(textarea) { min-width:0; max-width:100%; }
  :global(input[type=number]), :global(input[type=password]) { direction:ltr; text-align:start; }
  :global(:focus-visible) { outline:3px solid #b96026; outline-offset:3px; }
  :global(.row) { display:flex; flex-wrap:wrap; gap:1rem; } :global(.row > *) { flex:1; min-width:150px; }
  :global(.muted) { color:#526671; } footer { font-size:.85rem; color:#526671; }
</style>
