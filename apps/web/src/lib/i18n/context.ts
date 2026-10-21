import { getContext, setContext } from 'svelte';
import { translate } from './messages';
import type { Locale } from './locales';
const key = Symbol('request-locale');
export function provideLocale(locale: () => Locale) { setContext(key, locale); }
export function useI18n() {
  // Standalone component tests and previews default to English, never shared state.
  const locale = getContext<() => Locale>(key) ?? (() => 'en' as const);
  return { locale, t: (id: string, args?: Parameters<typeof translate>[2]) => translate(id, locale(), args) };
}
