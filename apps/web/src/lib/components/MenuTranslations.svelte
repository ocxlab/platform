<script lang="ts">
  import { useI18n } from '$lib/i18n/context';
  import type { Locale } from '$lib/i18n/locales';
  const { t } = useI18n();
  let { value = $bindable<Partial<Record<Locale, string>> | undefined>(), name }: { value?: Partial<Record<Locale, string>>; name: string } = $props();
  const languages = [{ id: 'de', name: 'Deutsch' }, { id: 'en', name: 'English' }, { id: 'ar', name: 'العربية' }] as const;
  function update(language: Locale, text: string) {
    const next = { ...value };
    if (text.trim()) next[language] = text; else delete next[language];
    value = Object.keys(next).length ? next : undefined;
  }
</script>
<details class="translations"><summary>{t('menu-translations', { name })}</summary>
  <p class="muted">{t('menu-translations-help')}</p>
  {#each languages as language}<label><bdi dir={language.id === 'ar' ? 'rtl' : 'ltr'}>{language.name}</bdi><input lang={language.id} dir={language.id === 'ar' ? 'rtl' : 'ltr'} maxlength="160" value={value?.[language.id] ?? ''} oninput={event => update(language.id, event.currentTarget.value)} /></label>{/each}
</details>
<style>.translations { margin-block: .5rem 1rem; } summary { cursor: pointer; color: #20586f; } </style>
