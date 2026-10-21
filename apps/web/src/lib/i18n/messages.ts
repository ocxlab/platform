import { FluentBundle, FluentResource } from '@fluent/bundle';
import { errorCodes, type ErrorCode } from './errors.generated';

export type Locale = 'en' | 'de';
const resources = import.meta.glob('./locales/*/*.ftl', { query: '?raw', import: 'default', eager: true }) as Record<string, string>;
const bundles = new Map<Locale, FluentBundle>();
for (const locale of ['en', 'de'] as const) {
  const bundle = new FluentBundle(locale);
  for (const [path, text] of Object.entries(resources)) {
    if (path.includes('/' + locale + '/')) {
      const errors = bundle.addResource(new FluentResource(text));
      if (errors.length) throw new Error('Invalid or duplicate Fluent resources: ' + path);
    }
  }
  bundles.set(locale, bundle);
}

export function localeFor(acceptLanguage = ''): Locale {
  const preferences = acceptLanguage.split(',').map((part) => {
    const [tag, ...params] = part.trim().toLowerCase().split(';');
    const quality = params.find((p) => p.trim().startsWith('q='))?.trim().slice(2);
    return { language: tag.split('-')[0], q: quality === undefined ? 1 : Number(quality) };
  }).filter((p) => Number.isFinite(p.q) && p.q > 0 && p.q <= 1).sort((a,b) => b.q - a.q);
  return preferences.find((p) => p.language === 'en' || p.language === 'de')?.language as Locale ?? 'en';
}

export function errorText(error: unknown, locale: Locale = 'en'): string {
  const code = error && typeof error === 'object' && 'code' in error ? error.code : undefined;
  const key = typeof code === 'string' && Object.hasOwn(errorCodes, code) ? errorCodes[code as ErrorCode].message_key : 'error-common-unavailable';
  const bundle = bundles.get(locale)!;
  const message = bundle.getMessage(key);
  if (!message?.value) return 'The service could not complete this request.';
  const errors: Error[] = [];
  const text = bundle.formatPattern(message.value, undefined, errors);
  return errors.length ? 'The service could not complete this request.' : text;
}

export function hasTranslation(locale: Locale, key: string): boolean { return Boolean(bundles.get(locale)?.getMessage(key)?.value); }
