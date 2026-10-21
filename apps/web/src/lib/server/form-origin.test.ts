import { describe, expect, it } from 'vitest';
import { forbiddenFormOrigin } from './form-origin';

const origin = 'https://shop.example';
describe('web form origin boundary', () => {
  it('allows safe methods without an origin', () => {
    for (const method of ['GET', 'HEAD', 'OPTIONS']) expect(forbiddenFormOrigin(new Request(origin, { method }), origin)).toBe(false);
  });
  it('rejects missing or foreign origins for unsafe form requests', () => {
    for (const method of ['POST', 'PUT', 'PATCH', 'DELETE']) {
      for (const type of ['application/x-www-form-urlencoded', 'multipart/form-data; boundary=test', 'text/plain; charset=UTF-8']) {
        for (const from of ['', 'https://untrusted.example', 'null']) {
          expect(forbiddenFormOrigin(new Request(origin, { method, headers: { 'content-type': type, ...(from ? { origin: from } : {}) } }), origin)).toBe(true);
        }
      }
    }
  });
  it('allows same-origin forms and leaves non-form requests to their own boundary', () => {
    expect(forbiddenFormOrigin(new Request(origin, { method: 'POST', headers: { 'content-type': 'application/x-www-form-urlencoded', origin } }), origin)).toBe(false);
    expect(forbiddenFormOrigin(new Request(origin, { method: 'POST', headers: { 'content-type': 'application/json', origin: 'https://other.example' } }), origin)).toBe(false);
  });
});
