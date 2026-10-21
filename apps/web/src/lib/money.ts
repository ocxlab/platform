import type { Locale } from '$lib/i18n/locales';
export const money = (minor: number, locale: Locale = 'de') => new Intl.NumberFormat(locale, { style: 'currency', currency: 'EUR' }).format(minor / 100);
