import { expect, test } from 'vitest';
import { money } from './money';
test('integer cents are formatted as euros', () => { expect(money(1400)).toContain('14,00'); expect(money(0)).toContain('0,00'); });
