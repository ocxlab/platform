import { expect, test } from 'vitest';
import { adminNavigation } from './admin-navigation';

const token = 'a'.repeat(64) + ':42';
test('keeps filters and size while moving forward and back without JavaScript', () => {
  const first = adminNavigation(new URL('http://localhost/admin?tab=orders&q=Pizza&status=accepted&limit=10'));
  expect(first.invalid).toBe(false);
  const second = adminNavigation(new URL(first.next(token), 'http://localhost'));
  expect(second.after).toBe(token);
  expect(second.previous).toBe(first.first);
  expect(second.q).toBe('Pizza');
  expect(second.status).toBe('accepted');
  expect(second.limit).toBe(10);
  expect(second.query).not.toContain('/inspect');
});
test('rejects malformed cursors, unsupported sizes and wrong-tab filters', () => {
  for (const query of ['after=bad','limit=10000','status=accepted','q='+ 'a'.repeat(161),'back=bad']) {
    expect(adminNavigation(new URL('http://localhost/admin?'+query)).invalid).toBe(true);
  }
});
test('bounds backwards-link history while retaining a route to the first page', () => {
  let navigation = adminNavigation(new URL('http://localhost/admin'));
  for (let i = 1; i <= 30; i++) navigation = adminNavigation(new URL(navigation.next('b'.repeat(64)+':'+i),'http://localhost'));
  expect(new URLSearchParams(navigation.query).getAll('back')).toHaveLength(20);
  expect(navigation.first).not.toContain('after=');
});
