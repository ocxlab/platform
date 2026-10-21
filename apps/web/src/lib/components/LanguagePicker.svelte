<script lang="ts">
  import { page } from '$app/state';
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import { languages } from '$lib/i18n/locales';
  import { useI18n } from '$lib/i18n/context';
  const { locale, t } = useI18n();
  const current = $derived(languages.find((language) => language.locale === locale())!);
  let picker: HTMLDetailsElement;
</script>
<svelte:window onkeydown={(event) => { if (event.key === 'Escape' && picker?.open && picker.contains(event.target as Node)) { picker.open = false; picker.querySelector('summary')?.focus(); } }}
  onclick={(event) => { if (picker?.open && !picker.contains(event.target as Node)) picker.open = false; }} />
<details class="language-picker" bind:this={picker}>
  <summary aria-label={t('ui-language')}>
    <img src={`/flags/${current.flag}.svg`} alt="" width="24" height="18" />
    <bdi lang={current.locale} dir={current.direction}>{current.name}</bdi>
    <ChevronDown size={16} aria-hidden="true" />
  </summary>
  <form method="POST" action="/language" aria-label={t('ui-language')}>
    <input type="hidden" name="return_to" value={page.url.pathname + page.url.search} />
    {#each languages as language}
      <button type="submit" name="locale" value={language.locale} aria-current={language.locale === locale() ? 'true' : undefined}>
        <img src={`/flags/${language.flag}.svg`} alt="" width="24" height="18" />
        <bdi lang={language.locale} dir={language.direction}>{language.name}</bdi>
      </button>
    {/each}
  </form>
</details>
<style>
  .language-picker { position: relative; }
  summary { display: flex; align-items: center; gap: .5rem; cursor: pointer; min-height: 44px; padding: .4rem .6rem; border: 1px solid #708692; border-radius: .3rem; background: white; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  form { position: absolute; inset-inline-end: 0; top: 100%; z-index: 30; min-width: 11rem; padding: .35rem; background: white; border: 1px solid #708692; border-radius: .3rem; box-shadow: 0 2px 6px #0003; }
  button { display: flex; align-items: center; gap: .7rem; width: 100%; margin: 0; min-height: 44px; background: white; color: #1a2b35; border-color: transparent; text-align: start; }
  button:hover, button[aria-current=true] { background: #e5eff3; }
  img { object-fit: cover; border: 1px solid #ccd8de; flex: none; }
  @media (max-width: 480px) {
    /* An in-flow disclosure cannot escape the viewport when the header wraps. */
    form { position: static; min-width: 0; margin-block-start: .35rem; }
  }
</style>
