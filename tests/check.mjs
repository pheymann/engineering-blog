import assert from 'node:assert/strict';
import { access, readFile, readdir } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const sourceDirectory = resolve(projectRoot, 'src/site');
const outputDirectory = resolve(projectRoot, 'dist');
const serviceUnit = resolve(projectRoot, 'deploy/engineering-blog.service');
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
  'a-small-deployment-pipeline-is-still-a-system/index.html',
  'what-i-want-from-a-local-first-engineering-blog/index.html',
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
  'ExecStartPre=/usr/bin/npm run build',
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

const installer = await readFile(installServiceScript, 'utf8');
for (const requiredCommand of [
  'useradd --system --user-group --home-dir /nonexistent --shell /usr/sbin/nologin',
  'install -d -o root -g "$service_user" -m 0750 "$application_directory"',
  'chown -R root:"$service_user" "$application_directory"',
  'install -o root -g root -m 0644 "$project_root/deploy/engineering-blog.service" /etc/systemd/system/engineering-blog.service'
]) {
  assert.match(installer, new RegExp(requiredCommand.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `installer needs ${requiredCommand}.`);
}

const tailscaleSetup = await readFile(tailscaleSetupScript, 'utf8');
assert.match(tailscaleSetup, /^tailscale serve --https=8443 --bg http:\/\/127\.0\.0\.1:8080$/m);
assert.doesNotMatch(tailscaleSetup, /tailscale funnel/);

const outputIndex = await readFile(resolve(outputDirectory, 'index.html'), 'utf8');
assert.match(outputIndex, /<link rel="stylesheet" href="\/assets\/styles\.css">/);
assert.match(outputIndex, /src="\/assets\/images\/pauls-engineering-blog\.svg"/);
assert.equal((outputIndex.match(/class="post-card"/g) ?? []).length, 2, 'Homepage must list exactly two posts.');
assert.match(outputIndex, /a-small-deployment-pipeline-is-still-a-system\//);
assert.match(outputIndex, /what-i-want-from-a-local-first-engineering-blog\//);
assert.match(outputIndex, /class="post-excerpt">(?:[^<]*<br>){4}/, 'Each excerpt must contain five lines.');

const sourceStyles = await readFile(resolve(sourceDirectory, 'assets/styles.css'), 'utf8');
assert.match(sourceStyles, /--color-page:/);
assert.match(sourceStyles, /--font-display:/);
assert.match(sourceStyles, /--side-margin:/);
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

for (const post of publicPages.slice(1, 3)) {
  const content = await readFile(resolve(outputDirectory, post), 'utf8');
  for (const element of ['<h1', '<h2', '<p', '<a ', '<li', '<img ', '<blockquote', '<sub', '<pre><code>']) {
    assert.match(content, new RegExp(element), `${post} must exercise ${element}.`);
  }
  assert.match(content, /<h1[\s\S]*?<\/h1><time/, `${post} must place the date directly below its title.`);
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
