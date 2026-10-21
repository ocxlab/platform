import { describe, expect, it } from 'vitest';
import { errorCodes } from './errors.generated';
import { errorText, hasTranslation, localeFor } from './messages';

describe('module-owned errors and Fluent messages', () => {
  it('has an English and German message for every public error code', () => {
    for (const entry of Object.values(errorCodes)) for (const locale of ['en','de'] as const) {
      expect(hasTranslation(locale,entry.message_key), `${locale}: ${entry.message_key}`).toBe(true);
    }
  });
  it('selects language by supported weighted preferences without shared mutable locale', () => {
    expect(localeFor('de-DE,de;q=0.9,en;q=0.8')).toBe('de');
    expect(localeFor('de;q=0,en;q=0.5')).toBe('en');
    expect(localeFor('fr,en;q=0.4,de;q=0.8')).toBe('de');
    expect(localeFor('fr')).toBe('en');
    expect(errorText({code:'quote_expired'},'de')).toContain('abgelaufen');
    expect(errorText({code:'quote_expired'},'en')).toContain('expired');
  });
  it('never displays backend details, exception strings, or unknown-code content', () => {
    for (const value of [{code:'unavailable',detail:'private SQL'}, {code:'invented_secret',detail:'private SQL'}, new Error('private SQL')]) {
      expect(errorText(value)).not.toContain('private');
      expect(errorText(value)).not.toContain('invented');
    }
  });
});
