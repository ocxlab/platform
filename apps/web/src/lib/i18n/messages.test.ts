import { describe, expect, it } from 'vitest';
import { errorCodes } from './errors.generated';
import { errorText, hasTranslation, localeFor, translate } from './messages';
import { languages, direction } from './locales';

describe('module-owned errors and Fluent messages', () => {
  it('has a message in every supported language for every public error code', () => {
    for (const entry of Object.values(errorCodes)) for (const { locale } of languages) {
      expect(hasTranslation(locale,entry.message_key), `${locale}: ${entry.message_key}`).toBe(true);
    }
  });
  it('selects language by supported weighted preferences without shared mutable locale', () => {
    expect(localeFor('de-DE,de;q=0.9,en;q=0.8')).toBe('de');
    expect(localeFor('de;q=0,en;q=0.5')).toBe('en');
    expect(localeFor('fr,en;q=0.4,de;q=0.8')).toBe('de');
    expect(localeFor('fr')).toBe('en');
    expect(localeFor('ar-EG,en;q=0.8')).toBe('ar');
    expect(direction('ar')).toBe('rtl');
    expect(direction('de')).toBe('ltr');
    expect(errorText({code:'quote_expired'},'de')).toContain('abgelaufen');
    expect(errorText({code:'quote_expired'},'en')).toContain('expired');
  });
  it('never displays backend details, exception strings, or unknown-code content', () => {
    for (const value of [{code:'unavailable',detail:'private SQL'}, {code:'invented_secret',detail:'private SQL'}, new Error('private SQL')]) {
      expect(errorText(value)).not.toContain('private');
      expect(errorText(value)).not.toContain('invented');
    }
  });
  it('keeps every UI message and interpolation available in all three languages', () => {
    const resources = import.meta.glob('./locales/*/*.ftl', { query: '?raw', import: 'default', eager: true }) as Record<string,string>;
    const byLocale = new Map<string, Set<string>>();
    const args = { name: 'Döner', published: 1, draft: 2, min: 1, max: 2, amount: '€15.00', time: '12:00', generation: 1, revision: 1, visibility: 'ok' };
    for (const { locale } of languages) {
      const keys = new Set<string>();
      for (const [path, source] of Object.entries(resources)) if (path.includes('/'+locale+'/')) {
        for (const match of source.matchAll(/^([a-z][a-z0-9-]+)\s*=/gm)) {
          expect(keys.has(match[1]), 'Duplicate key '+match[1]).toBe(false);
          keys.add(match[1]);
          expect(translate(match[1], locale, args), locale+': '+match[1]).not.toBe('…');
        }
      }
      byLocale.set(locale, keys);
    }
    for (const { locale } of languages) expect([...byLocale.get(locale)!].sort()).toEqual([...byLocale.get('en')!].sort());
    expect(translate('editor-remove','ar',{name:'Döner'})).toContain('\u2068Döner\u2069');
    const sources = import.meta.glob(['/src/routes/**/*.svelte','/src/routes/**/*.ts','/src/lib/components/*.svelte'], { query: '?raw', import: 'default', eager: true }) as Record<string,string>;
    for (const [path, source] of Object.entries(sources)) if (!path.endsWith('.test.ts')) {
      for (const match of source.matchAll(/\b(?:t|text)\('([a-z][a-z0-9-]+)'/g)) expect(byLocale.get('en')!.has(match[1]), path+': '+match[1]).toBe(true);
    }
  });
});
