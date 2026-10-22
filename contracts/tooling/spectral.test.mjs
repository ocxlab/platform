import assert from 'node:assert/strict';
import { test } from 'node:test';
import { mkdtemp, writeFile, rm, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, resolve, join } from 'node:path';
import { createRequire } from 'node:module';
import { spawnSync } from 'node:child_process';
import objectRequest from './functions/objectRequest.js';
import problemResponses from './functions/problemResponses.js';

const context = { path: ['paths', '/orders', 'post', 'requestBody'] };
const request = schema => ({ content: { 'application/json': { schema } } });
for (const [name, schema, accepted] of [
  ['object', { type: 'object' }, true],
  ['array', { type: 'array', items: { type: 'object' } }, false],
  ['string', { type: 'string' }, false],
  ['integer', { type: 'integer' }, false],
  ['unconstrained', {}, false],
  ['nullable object', { type: 'object', nullable: true }, false],
  ['allOf object', { allOf: [{ type: 'object' }, { required: ['id'] }] }, true],
  ['allOf without a root type', { allOf: [{ properties: { id: { type: 'string' } } }] }, false],
  ['object union', { oneOf: [{ type: 'object' }, { type: 'object' }] }, true],
  ['mixed union', { anyOf: [{ type: 'object' }, { type: 'array' }] }, false],
  ['typed object with property constraints', { type: 'object', anyOf: [{ required: ['id'] }, { required: ['name'] }] }, true],
]) {
  test(`request root: ${name}`, () => assert.equal(!objectRequest(request(schema), null, context), accepted));
}
test('request media must be application/json only', () => {
  assert.ok(objectRequest({ content: { 'text/plain': { schema: { type: 'object' } } } }, null, context));
  assert.ok(objectRequest({}, null, context));
});
test('error rule handles default, wildcard, redirects, SSE, and extra media types', () => {
  const good = { content: { 'application/problem+json': { schema: { type: 'object' } } } };
  assert.deepEqual(problemResponses({ default: good, '400': good, '5XX': good, '307': {}, '204': {}, '200': { content: { 'text/event-stream': { schema: { type: 'string' } } } } }, null, context), []);
  const findings = problemResponses({ '404': {}, default: { content: { 'application/json': { schema: {} } } }, '4XX': { content: { ...good.content, 'text/plain': { schema: { type: 'string' } } } } }, null, context);
  assert.equal(findings.length, 3);
});

test('CLI rules resolve local refs, allow valid APIs, and reject bad design', async () => {
  const require = createRequire(import.meta.url);
  const pkgPath = require.resolve('@stoplight/spectral-cli/package.json');
  const pkg = JSON.parse(await readFile(pkgPath, 'utf8'));
  const cli = resolve(dirname(pkgPath), typeof pkg.bin === 'string' ? pkg.bin : pkg.bin.spectral);
  const directory = await mkdtemp(join(tmpdir(), 'ocp-spectral-'));
  try {
    const schemaFile = join(directory, 'schemas.json');
    await writeFile(schemaFile, JSON.stringify({ Body: { type: 'object', properties: { note: { type: 'string' } } }, ArrayBody: { type: 'array', items: { type: 'string' } }, Problem: { type: 'object' } }));
    const problem = { description: 'Safe problem details.', content: { 'application/problem+json': { schema: { $ref: './schemas.json#/Problem' } } } };
    const operation = {
      operationId: 'CreateOrder', summary: 'Create an order', description: 'Retain agreed terms for explicit order placement.', tags: ['Commerce'], 'x-access': 'public', security: [],
      parameters: [{ name: 'storeId', in: 'path', required: true, description: 'Store UUID.', schema: { type: 'string' } }, { name: 'pageSize', in: 'query', description: 'Bounded page size.', schema: { type: 'integer' } }, { name: 'Idempotency-Key', in: 'header', description: 'Scoped retry key.', schema: { type: 'string' } }],
      requestBody: request({ $ref: './schemas.json#/Body' }),
      responses: { '201': { description: 'Created.', content: { 'application/json': { schema: { type: 'object' } } } }, '400': problem, default: problem },
    };
    const valid = { openapi: '3.0.3', info: { title: 'Rule fixture', version: '1.0.0', description: 'Fictional API lint acceptance fixture.', contact: { name: 'OCP Lab' } }, servers: [{ url: '/' }], tags: [{ name: 'Commerce', description: 'Purchase interactions.' }], paths: { '/stores/{storeId}/orders': { post: operation } } };
    function lint(file) {
      const result = spawnSync(process.execPath, [cli, 'lint', file, '--ruleset', resolve('../../.spectral.yaml'), '--format', 'json', '--quiet', '--fail-severity', 'warn'], { encoding: 'utf8' });
      assert.ifError(result.error);
      assert.ok(result.stdout.trim().startsWith('['), result.stderr || result.stdout);
      return { status: result.status, findings: JSON.parse(result.stdout) };
    }
    const file = join(directory, 'api.json');
    await writeFile(file, JSON.stringify(valid));
    const accepted = lint(file);
    assert.equal(accepted.status, 0, JSON.stringify(accepted.findings));
    assert.deepEqual(accepted.findings, []);
    const broken = structuredClone(valid);
    const bad = broken.paths['/stores/{storeId}/orders'].post;
    bad.parameters[0].name = 'store_id';
    bad.requestBody = request({ $ref: './schemas.json#/ArrayBody' });
    bad.responses['400'] = { description: 'Wrong media.', content: { 'application/json': { schema: { type: 'object' } } } };
    delete bad['x-access'];
    broken.paths['/Bad_path/'] = { get: { ...structuredClone(operation), operationId: 'BadPath', requestBody: request({ type: 'object' }) } };
    broken.paths['/get/orders?sort=desc'] = { get: { ...structuredClone(operation), operationId: 'BadQuery' } };
    await writeFile(file, JSON.stringify(broken));
    const rejected = lint(file);
    assert.equal(rejected.status, 1);
    const codes = new Set(rejected.findings.map(finding => finding.code));
    for (const rule of ['paths-kebab-case', 'paths-no-trailing-slash', 'paths-no-http-verbs', 'paths-no-query-params', 'parameters-camel-case', 'request-body-must-be-object', 'error-response-must-use-problem-json', 'operation-access-required', 'get-no-request-body']) {
      assert.ok(codes.has(rule), `Missing ${rule}: ${JSON.stringify(rejected.findings)}`);
    }
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
