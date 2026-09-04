import { createReadStream } from 'node:fs';
import { access, stat } from 'node:fs/promises';
import { createServer } from 'node:http';
import { dirname, extname, join, normalize, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const siteDirectory = resolve(process.env.SITE_DIRECTORY ?? join(projectRoot, 'dist'));
const host = process.env.HOST ?? '127.0.0.1';
const port = Number(process.env.PORT ?? 8080);
const mimeTypes = {
  '.css': 'text/css; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.txt': 'text/plain; charset=utf-8',
  '.xml': 'application/xml; charset=utf-8',
  '.woff2': 'font/woff2'
};

function requestedFile(urlPath) {
  let decodedPath;
  try {
    decodedPath = decodeURIComponent(urlPath);
  } catch {
    return undefined;
  }
  const relativePath = normalize(decodedPath).replace(/^[/\\]+/, '');
  const candidate = resolve(siteDirectory, relativePath || 'index.html');

  if (!candidate.startsWith(`${siteDirectory}/`) && candidate !== siteDirectory) {
    return null;
  }

  return candidate;
}

const server = createServer(async (request, response) => {
  let urlPath;
  try {
    // A fixed base avoids making request parsing depend on an untrusted Host header.
    urlPath = new URL(request.url ?? '/', 'http://localhost').pathname;
  } catch {
    response.writeHead(400, { Connection: 'close' }).end('Bad request');
    return;
  }

  const filePath = requestedFile(urlPath);

  if (filePath === undefined) {
    response.writeHead(400, { Connection: 'close' }).end('Bad request');
    return;
  }

  if (!filePath) {
    response.writeHead(403).end('Forbidden');
    return;
  }

  let resolvedFilePath = filePath;
  try {
    if ((await stat(resolvedFilePath)).isDirectory()) {
      resolvedFilePath = join(resolvedFilePath, 'index.html');
    }
    await access(resolvedFilePath);
  } catch {
    response.writeHead(404).end('Not found');
    return;
  }

  response.writeHead(200, {
    'Content-Type': mimeTypes[extname(resolvedFilePath)] ?? 'application/octet-stream',
    'X-Content-Type-Options': 'nosniff'
  });
  createReadStream(resolvedFilePath).pipe(response);
});

server.listen(port, host, () => {
  console.log(`Serving ${siteDirectory} at http://${host}:${port}`);
});
