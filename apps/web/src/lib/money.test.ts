import { expect, test } from 'vitest';
import { money } from './money';
test('integer cents are formatted as euros', () => { expect(money(1400)).toContain('14,00'); expect(money(0)).toContain('0,00'); });
test('formatting changes presentation without changing the EUR amount', () => {
  expect(money(1400,'en')).toContain('14.00');
  expect(money(1400,'de')).toContain('14,00');
  // CLDR/ICU versions differ on Arabic's default numbering system.
  expect(money(1400,'ar')).toMatch(/14\.00|١٤٫٠٠/);
  for (const locale of ['en','de','ar'] as const) expect(money(1400,locale)).toContain('€');
});
