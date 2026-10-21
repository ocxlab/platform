export const languages = [
  { locale: 'de', name: 'Deutsch', direction: 'ltr', flag: 'de' },
  { locale: 'en', name: 'English', direction: 'ltr', flag: 'gb' },
  { locale: 'ar', name: 'العربية', direction: 'rtl', flag: 'sa' },
] as const;
export type Locale = typeof languages[number]['locale'];
export function isLocale(value: unknown): value is Locale {
  return languages.some((language) => language.locale === value);
}
export function direction(locale: Locale): 'ltr' | 'rtl' { return locale === 'ar' ? 'rtl' : 'ltr'; }
export function localeFor(acceptLanguage = ''): Locale {
  const preferences = acceptLanguage.split(',').map((part) => {
    const [tag, ...params] = part.trim().toLowerCase().split(';');
    const quality = params.find((p) => p.trim().startsWith('q='))?.trim().slice(2);
    return { language: tag.split('-')[0], q: quality === undefined ? 1 : Number(quality) };
  }).filter((p) => Number.isFinite(p.q) && p.q > 0 && p.q <= 1).sort((a,b) => b.q - a.q);
  return preferences.find((p) => isLocale(p.language))?.language as Locale ?? 'en';
}
