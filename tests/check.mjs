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
const githubPagesGuide = resolve(projectRoot, 'deploy/github-pages.md');
const publicVerificationScript = resolve(projectRoot, 'scripts/verify-github-pages.sh');
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
  'Environment=BLOG_PRODUCTION_STATE_PATH=/var/lib/engineering-blog/production-state.json',
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
  'chown root:"$service_user" "$application_directory"',
  'go build -o bin/vault-preview-service ./cmd/vault-preview-service',
  'install -o root -g root -m 0644 "$project_root/deploy/engineering-blog.service" "$unit_directory/engineering-blog.service"',
  'install -o root -g root -m 0644 "$project_root/deploy/engineering-blog-preview.service" "$unit_directory/engineering-blog-preview.service"'
]) {
  assert.match(installer, new RegExp(requiredCommand.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `installer needs ${requiredCommand}.`);
}
assert.doesNotMatch(installer, /find "\$application_directory" -type [df] -exec chmod/, 'installer must preserve Git-recorded file modes.');
for (const requiredSafetyCheck of [
  '"$systemctl_command" stop engineering-blog-preview.service 2>/dev/null || true',
  'git -C "$application_directory" fetch origin main',
  'git -C "$application_directory" merge-base --is-ancestor HEAD FETCH_HEAD',
  'refusing to replace checkout: local commits are not on origin/main',
  'refusing to replace checkout with uncommitted changes',
  'refusing to replace non-Git application directory $application_directory',
  'is_known_legacy_layout',
  'ENGINEERING_BLOG_APPLICATION_DIRECTORY',
  'ENGINEERING_BLOG_STATE_DIRECTORY',
  'restore_publisher_on_failure'
]) {
  assert.match(installer, new RegExp(requiredSafetyCheck.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `installer needs ${requiredSafetyCheck}.`);
}

const tailscaleSetup = await readFile(tailscaleSetupScript, 'utf8');
assert.match(tailscaleSetup, /^tailscale serve --https=8443 --bg http:\/\/127\.0\.0\.1:8080$/m);
assert.doesNotMatch(tailscaleSetup, /tailscale funnel/);

const outputIndex = await readFile(resolve(outputDirectory, 'index.html'), 'utf8');
assert.match(outputIndex, /<link rel="stylesheet" href="\/assets\/styles\.css">/);
assert.match(outputIndex, /src="\/assets\/images\/pauls-engineering-blog\.svg"/);
assert.equal((outputIndex.match(/class="post-card"/g) ?? []).length, 0, 'Checked-in homepage must not contain mock posts.');
assert.doesNotMatch(outputIndex, /Latest engineering notes|class="section-title"/, 'Checked-in homepage must not retain the removed latest-post heading.');
assert.match(outputIndex, /<p class="homepage-intro">Hi, I am Paul and I am a Staff Engineer and Engineering Manager from Germany\. Talk to me on <a href="https:\/\/www\.linkedin\.com\/in\/paul-heymann-6b4a53144\/">LinkedIn<\/a> or send me an email at <a href="mailto:contact@paulheymann\.de">contact@paulheymann\.de<\/a>\.<\/p>/, 'Checked-in homepage must include the linked introduction.');
assert.ok(outputIndex.indexOf('class="homepage-intro"') < outputIndex.indexOf('class="post-list"'), 'Homepage introduction must appear before the post list.');

const sourceIndex = await readFile(resolve(sourceDirectory, 'index.html'), 'utf8');
assert.doesNotMatch(sourceIndex, /Latest engineering notes|class="section-title"/, 'Homepage source must not retain the removed latest-post heading.');
assert.match(sourceIndex, /href="mailto:contact@paulheymann\.de"/, 'Homepage source email address must be a functional mailto link.');

const sourceStyles = await readFile(resolve(sourceDirectory, 'assets/styles.css'), 'utf8');
assert.match(sourceStyles, /--color-page:/);
assert.match(sourceStyles, /--font-display:/);
assert.match(sourceStyles, /--side-margin:/);
assert.match(sourceStyles, /body\s*\{[^}]*display:\s*flex;[^}]*min-height:\s*100vh;[^}]*flex-direction:\s*column;/s);
assert.match(sourceStyles, /\.site-content\s*\{[^}]*flex:\s*1;/s);
assert.match(sourceStyles, /\.homepage-intro\s*\{[^}]*margin:\s*0 0 var\(--space-4\);/s, 'Homepage introduction must be separated from the post list.');
assert.match(sourceStyles, /@media \(max-width: 42rem\)/);
assert.match(sourceStyles, /\.post\s*\{[^}]*max-width:\s*700px;/s, 'Post bodies must be capped at 700px.');
assert.doesNotMatch(sourceStyles, /\.section-title(?:\s|:|\{)/, 'Homepage heading styles must be removed when unused.');
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

const sourceCname = await readFile(resolve(sourceDirectory, 'CNAME'), 'utf8');
const outputCname = await readFile(resolve(outputDirectory, 'CNAME'), 'utf8');
assert.equal(sourceCname, 'engineering.paulheymann.de\n', 'The Pages custom-domain source must be exact.');
assert.equal(outputCname, sourceCname, 'The built site must retain the Pages custom-domain file.');

const logo = await readFile(resolve(sourceDirectory, 'assets/images/pauls-engineering-blog.svg'), 'utf8');
assert.match(logo, /<title id="title">Paul's Engineering Blog<\/title>/, 'Logo needs an accessible title.');
assert.match(logo, /<desc id="description">/, 'Logo needs an accessible description.');
assert.match(logo, /aria-labelledby="title description"/, 'Logo must expose its title and description.');
assert.match(logo, />Paul's<\/text>/, 'The rainbow wordmark must visibly include the apostrophe.');
assert.doesNotMatch(
  logo,
  /letter-spacing="-\d+"[^>]*>Paul's<\/text>/,
  'The visible apostrophe must not be merged into the final letter by negative tracking.'
);
assert.match(logo, /<text\b[^>]*fill="#111111"[^>]*>ENGINEERING BLOG<\/text>/, 'Engineering Blog must remain black.');
assert.match(logo, /viewBox="0 0 1050 145"/, 'The logo canvas must leave room for a single-line title.');

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

const impressum = await readFile(resolve(outputDirectory, 'impressum/index.html'), 'utf8');
assert.match(impressum, /<h1>Impressum<\/h1>/, 'The Impressum needs a page heading.');
assert.match(impressum, /<h2 id="angaben-gemaess-5-ddg">Angaben gemäß § 5 DDG<\/h2>/, 'The provider information must cite the current German Digital Services Act.');
assert.doesNotMatch(impressum, /\bTMG\b/, 'The Impressum must not cite the superseded Telemedia Act.');
assert.match(impressum, /Paul Heymann<br>Hilleborchstraße 3<br>38855 Wernigerode/, 'The Impressum must include the supplied postal address.');
assert.match(impressum, /<a href="mailto:contact@paulheymann\.de">contact@paulheymann\.de<\/a>/, 'The Impressum email address must be a functional mailto link.');
assert.doesNotMatch(impressum, /This page is a placeholder\./, 'The Impressum must not retain its placeholder text.');

const privacy = await readFile(resolve(outputDirectory, 'datenschutzerklaerung/index.html'), 'utf8');
assert.match(privacy, /<h1>Datenschutzerklärung<\/h1>/, 'The privacy notice needs a page heading.');
assert.match(privacy, /<meta name="description" content="[^"]*Datenschutzerklärung[^\"]*(?:für|zum|des|von)[^\"]*">/, 'The German privacy notice needs a German meta description.');
assert.match(privacy, /<meta property="og:description" content="[^"]*Datenschutzerklärung[^\"]*(?:für|zum|des|von)[^\"]*">/, 'The German privacy notice needs a German Open Graph description.');
assert.doesNotMatch(privacy, /This page is a placeholder\./, 'The privacy notice must not retain its placeholder text.');
assert.match(privacy, /Paul Heymann<br>Hilleborchstraße 3<br>38855 Wernigerode/, 'The privacy notice must identify the controller and postal address.');
assert.match(privacy, /mailto:contact@paulheymann\.de/, 'The privacy notice must provide a contact email address.');
assert.match(privacy, /GitHub Pages/, 'The privacy notice must name its delivery provider.');
assert.doesNotMatch(privacy, /Cloudflare/, 'The privacy notice must not name the retired delivery provider.');
assert.match(privacy, /IP-Adresse/, 'The privacy notice must explain necessary request data.');
assert.match(privacy, /Art\. 6 Abs\. 1 lit\. f DSGVO/, 'The privacy notice must state its legal basis.');
assert.match(privacy, /https:\/\/docs\.github\.com\/site-policy\/privacy-policies\/github-privacy-statement/, 'The privacy notice must link GitHub’s privacy statement.');
assert.match(privacy, /keine Cookies, keine Reichweitenmessung, keine Werbung, kein Tracking, keine Formulare, keine Benutzerkonten und kein clientseitiges JavaScript/, 'The privacy notice must accurately state this site’s additional data practices.');
assert.match(privacy, /Recht auf Auskunft, Berichtigung, Löschung, Einschränkung der Verarbeitung, Datenübertragbarkeit sowie Widerspruch/, 'The privacy notice must explain data-subject rights.');
assert.match(privacy, /Datenschutz-Aufsichtsbehörde/, 'The privacy notice must explain the right to complain.');

const pagesGuide = await readFile(githubPagesGuide, 'utf8');
assert.match(pagesGuide, /failed-push recovery/i, 'The Pages guide must cover failed-push recovery.');
assert.match(pagesGuide, /credential\s+rotation/i, 'The Pages guide must cover credential rotation.');
const publicVerification = await readFile(publicVerificationScript, 'utf8');
assert.match(publicVerification, /BLOG_PUBLIC_URL/, 'Public verification must be opt-in.');

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
