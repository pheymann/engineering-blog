import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createConnection } from 'node:net';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';
import test from 'node:test';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');

test('malformed request paths do not terminate the static server', async (context) => {
  const server = spawn(process.execPath, ['scripts/serve.mjs'], {
    cwd: projectRoot,
    env: { ...process.env, PORT: '18081' },
    stdio: ['ignore', 'pipe', 'pipe']
  });
  context.after(() => {
    if (server.exitCode === null) server.kill();
  });

  await once(server.stdout, 'data');

  const client = createConnection({ host: '127.0.0.1', port: 18081 });
  await once(client, 'connect');
  let response = '';
  client.setEncoding('utf8');
  client.on('data', (chunk) => {
    response += chunk;
  });
  client.end('GET /% HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n');
  await once(client, 'close');

  assert.match(response, /^HTTP\/1\.1 400 Bad Request\r?\n/m, 'a malformed request must receive a bad-request response');
  await new Promise((resolveWait) => setTimeout(resolveWait, 100));
  assert.equal(server.exitCode, null, 'a malformed request must receive an error response without crashing the server');
});
