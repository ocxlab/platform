import { FluentBundle, FluentResource } from '@fluent/bundle';
import type { FluentVariable } from '@fluent/bundle';
import { errorCodes, type ErrorCode } from './errors.generated';
import { languages, type Locale } from './locales';
export { localeFor, type Locale } from './locales';

const resources = import.meta.glob('./locales/*/*.ftl', { query: '?raw', import: 'default', eager: true }) as Record<string, string>;
const bundles = new Map<Locale, FluentBundle>();
for (const { locale } of languages) {
  const bundle = new FluentBundle(locale);
  for (const [path, text] of Object.entries(resources)) {
    if (path.includes('/' + locale + '/')) {
      const errors = bundle.addResource(new FluentResource(text));
      if (errors.length) throw new Error('Invalid or duplicate Fluent resources: ' + path);
    }
  }
  bundles.set(locale, bundle);
}

export function translate(key: string, locale: Locale = 'en', args?: Record<string, FluentVariable>): string {
  const bundle = bundles.get(locale)!;
  const message = bundle.getMessage(key);
  if (!message?.value) return '…';
  const errors: Error[] = [];
  const text = bundle.formatPattern(message.value, args, errors);
  return errors.length ? '…' : text;
}

export function errorText(error: unknown, locale: Locale = 'en'): string {
  const code = error && typeof error === 'object' && 'code' in error ? error.code : undefined;
  const key = typeof code === 'string' && Object.hasOwn(errorCodes, code) ? errorCodes[code as ErrorCode].message_key : 'error-common-unavailable';
  return translate(key, locale);
}

export function hasTranslation(locale: Locale, key: string): boolean { return Boolean(bundles.get(locale)?.getMessage(key)?.value); }
