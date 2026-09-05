import assert from 'node:assert/strict';
import { access, readFile, readdir } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const sourceDirectory = resolve(projectRoot, 'src/site');
const outputDirectory = resolve(projectRoot, 'dist');
const serviceUnit = resolve(projectRoot, 'deploy/engineering-blog.service');
const previewServiceUnit = resolve(projectRoot, 'deploy/engineering-blog-preview.service');
const installServiceScript = resolve(projectRoot, 'deploy/install-service.sh');
const tailscaleSetupScript = resolve(projectRoot, 'deploy/configure-tailscale-serve.sh');
const bannedPatterns = [
  /analytics/i,
  /googletagmanager/i,
  /google-analytics/i,
  /plausible/i,
  /matomo/i
];
const textExtensions = new Set(['.css', '.html', '.svg']);
const publicPages = [
  'index.html',
  'impressum/index.html',
  'datenschutzerklaerung/index.html'
];
const canonicalUrls = publicPages.map((page) => page === 'index.html'
  ? 'https://engineering.paulheymann.de/'
  : `https://engineering.paulheymann.de/${page.replace(/index\.html$/, '')}`);

async function filesIn(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const paths = await Promise.all(entries.map(async (entry) => {
    const entryPath = resolve(directory, entry.name);
    return entry.isDirectory() ? filesIn(entryPath) : [entryPath];
  }));
  return paths.flat();
}

const sourceFiles = await filesIn(sourceDirectory);
const outputFiles = await filesIn(outputDirectory);
assert.ok(sourceFiles.length > 0, 'The checked-in site source must not be empty.');
assert.equal(outputFiles.length, sourceFiles.length, 'The build output must mirror all source assets.');

const unit = await readFile(serviceUnit, 'utf8');
for (const requiredDirective of [
  'User=engineering-blog',
  'Group=engineering-blog',
  'WorkingDirectory=/opt/engineering-blog',
  'Environment=HOME=/var/lib/engineering-blog',
  'Environment=HOST=127.0.0.1',
  'Environment=PORT=8080',
  'Environment=SITE_DIRECTORY=/var/lib/engineering-blog/site',
  'StateDirectory=engineering-blog',
  'ExecStart=/usr/bin/node scripts/serve.mjs',
  'Restart=on-failure',
  'StandardOutput=journal',
  'StandardError=journal',
  'ProtectSystem=strict',
  'NoNewPrivileges=yes',
  'ProtectHome=yes',
  'IPAddressDeny=any',
  'IPAddressAllow=127.0.0.1/32'
]) {
  assert.match(unit, new RegExp(`^${requiredDirective.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`, 'm'), `service unit needs ${requiredDirective}.`);
}
assert.doesNotMatch(unit, /\/root\/engineering-blog/, 'the service must not depend on the root-owned checkout at runtime.');

const previewUnit = await readFile(previewServiceUnit, 'utf8');
for (const requiredDirective of [
  'User=root',
  'Environment="BLOG_VAULT_DIRECTORY=/root/obsidian-vault/Engineering Blog"',
  'Environment=BLOG_PREVIEW_OUTPUT_DIRECTORY=/var/lib/engineering-blog/site',
  'ExecStartPre=/opt/engineering-blog/bin/vault-preview-service --build-once',
  'ExecStart=/opt/engineering-blog/bin/vault-preview-service',
  'Restart=on-failure',
  'StandardOutput=journal',
  'StandardError=journal',
  'ProtectSystem=strict',
  'ProtectHome=read-only'
]) {
  assert.match(previewUnit, new RegExp(`^${requiredDirective.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`, 'm'), `preview unit needs ${requiredDirective}.`);
}

const installer = await readFile(installServiceScript, 'utf8');
for (const requiredCommand of [
  'useradd --system --user-group --home-dir /nonexistent --shell /usr/sbin/nologin',
  'install -d -o root -g "$service_user" -m 0750 "$application_directory"',
  'chown -R root:"$service_user" "$application_directory"',
  'go build -o bin/vault-preview-service ./cmd/vault-preview-service',
  'install -o root -g root -m 0644 "$project_root/deploy/engineering-blog.service" /etc/systemd/system/engineering-blog.service',
  'install -o root -g root -m 0644 "$project_root/deploy/engineering-blog-preview.service" /etc/systemd/system/engineering-blog-preview.service'
]) {
  assert.match(installer, new RegExp(requiredCommand.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `installer needs ${requiredCommand}.`);
}

const tailscaleSetup = await readFile(tailscaleSetupScript, 'utf8');
assert.match(tailscaleSetup, /^tailscale serve --https=8443 --bg http:\/\/127\.0\.0\.1:8080$/m);
assert.doesNotMatch(tailscaleSetup, /tailscale funnel/);

const outputIndex = await readFile(resolve(outputDirectory, 'index.html'), 'utf8');
assert.match(outputIndex, /<link rel="stylesheet" href="\/assets\/styles\.css">/);
assert.match(outputIndex, /src="\/assets\/images\/pauls-engineering-blog\.svg"/);
assert.equal((outputIndex.match(/class="post-card"/g) ?? []).length, 0, 'Checked-in homepage must not contain mock posts.');

const sourceStyles = await readFile(resolve(sourceDirectory, 'assets/styles.css'), 'utf8');
assert.match(sourceStyles, /--color-page:/);
assert.match(sourceStyles, /--font-display:/);
assert.match(sourceStyles, /--side-margin:/);
assert.match(sourceStyles, /body\s*\{[^}]*display:\s*flex;[^}]*min-height:\s*100vh;[^}]*flex-direction:\s*column;/s);
assert.match(sourceStyles, /\.site-content\s*\{[^}]*flex:\s*1;/s);
assert.match(sourceStyles, /@media \(max-width: 42rem\)/);
assert.doesNotMatch(sourceStyles, /border-radius\s*:/);

for (const fontFile of [
  'press-start-2p-latin-400-normal.woff2',
  'inter-tight-latin-400-normal.woff2',
  'inter-tight-latin-600-normal.woff2'
]) {
  await access(resolve(sourceDirectory, 'assets/fonts', fontFile));
}

await access(resolve(sourceDirectory, 'assets/images/pauls-engineering-blog.svg'));
await access(resolve(sourceDirectory, 'assets/images/static-publishing-path.svg'));
await access(resolve(sourceDirectory, 'robots.txt'));
await access(resolve(sourceDirectory, 'sitemap.xml'));

const robots = await readFile(resolve(outputDirectory, 'robots.txt'), 'utf8');
assert.match(robots, /^User-agent: \*\nAllow: \/\nSitemap: https:\/\/engineering\.paulheymann\.de\/sitemap\.xml$/m);
const sitemap = await readFile(resolve(outputDirectory, 'sitemap.xml'), 'utf8');
for (const url of canonicalUrls) {
  assert.match(sitemap, new RegExp(`<loc>${url.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}</loc>`), `${url} must appear in the sitemap.`);
}

for (const [index, page] of publicPages.entries()) {
  const content = await readFile(resolve(outputDirectory, page), 'utf8');
  assert.match(content, new RegExp(`<link rel="canonical" href="${canonicalUrls[index]}">`), `${page} needs a canonical URL.`);
  assert.match(content, /<meta property="og:title"/, `${page} needs an Open Graph title.`);
  assert.match(content, /<meta property="og:type"/, `${page} needs an Open Graph type.`);
  assert.match(content, /<meta property="og:url"/, `${page} needs an Open Graph URL.`);
  assert.match(content, /<meta property="og:description"/, `${page} needs an Open Graph excerpt.`);
  assert.doesNotMatch(content, /<script\b/i, `${page} must not require JavaScript.`);
  assert.doesNotMatch(content, /document\.cookie/i, `${page} must not set cookies.`);
}

for (const filePath of [...sourceFiles, ...outputFiles]) {
  if (!textExtensions.has(filePath.slice(filePath.lastIndexOf('.')))) {
    continue;
  }
  const content = await readFile(filePath, 'utf8');
  assert.doesNotMatch(content, /src=["']https?:\/\//i, `${filePath} must not load remote assets.`);
  for (const pattern of bannedPatterns) {
    assert.doesNotMatch(content, pattern, `${filePath} must not load remote assets or analytics.`);
  }
}

console.log(`Checked ${sourceFiles.length} source assets and ${outputFiles.length} generated assets.`);
