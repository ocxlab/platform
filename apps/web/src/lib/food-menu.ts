import type { components } from '$lib/api/generated';
import type { Locale } from '$lib/i18n/locales';

export type FoodMenu = components['schemas']['FoodMenu'];
export type BasketLine = components['schemas']['FoodSelectionRequest']['lines'][number];
export function menuName(value: { name: string; translations?: Partial<Record<Locale, string>> }, locale: Locale) {
  return value.translations?.[locale] || value.name;
}
// Labels and client totals are presentation only. The quote API rechecks every ID,
// choice, quantity and price against the current publication.
export function basketPrice(menu: FoodMenu, line: BasketLine) {
  const item = menu.items.find(item => item.id === line.item_id);
  const variant = item?.variants.find(variant => variant.id === line.variant_id);
  if (!item || !variant) return 0;
  return (variant.price_minor + line.options.reduce((total, selected) => total + (item.option_groups.find(g => g.id === selected.group_id)?.options.find(o => o.id === selected.option_id)?.price_minor ?? 0), 0)) * line.quantity;
}
export function validBasketLine(value: unknown): value is BasketLine {
  if (!value || typeof value !== 'object') return false;
  const line = value as BasketLine;
  const id = (v: unknown) => typeof v === 'string' && /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/.test(v);
  return id(line.item_id) && id(line.variant_id) && Number.isInteger(line.quantity) && line.quantity >= 1 && line.quantity <= 99
    && (line.note === undefined || typeof line.note === 'string' && [...line.note].length <= 500)
    && Array.isArray(line.options) && line.options.length <= 600 && line.options.every(o => o && id(o.group_id) && id(o.option_id));
}
export function basketSignature(lines: BasketLine[], note: string) {
  return JSON.stringify({ lines: lines.map(line => ({ item_id: line.item_id, variant_id: line.variant_id, quantity: line.quantity,
    options: [...line.options].sort((a, b) => `${a.group_id}:${a.option_id}`.localeCompare(`${b.group_id}:${b.option_id}`)), note: line.note || '' })), note });
}
