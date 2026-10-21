import { spawn } from 'node:child_process';
let input = '';
for await (const chunk of process.stdin) input += chunk;
let fixture;
try { fixture = JSON.parse(input); } catch { throw new Error('Disposable browser fixture was not supplied'); }
if (!fixture.merchant_id || !fixture.merchant_token || !fixture.provider_token) throw new Error('Incomplete browser fixture');
const child = spawn(process.execPath, ['node_modules/@playwright/test/cli.js', 'test'], {
  env: { ...process.env, OCP_BROWSER_FIXTURE: JSON.stringify(fixture) }, stdio: ['ignore', 'inherit', 'inherit'],
});
child.on('error', () => { process.exitCode = 1; });
child.on('exit', (code) => { process.exitCode = code ?? 1; });
