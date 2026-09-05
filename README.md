# Paul's Engineering Blog

This repository is the MVP for Paul's Engineering Blog. It is a static, privacy-focused preview of `engineering.paulheymann.de`: plain HTML and CSS, local fonts and images, no JavaScript, cookies, analytics, or external runtime downloads.

The repository contains the responsive site design, local vault-to-HTML preview pipeline, private Tailscale HTTPS preview, and automated checks. It does **not** publish to Cloudflare; that remains outside this milestone.

The Go vault-preview service parses and validates tagged Markdown, builds the local preview, and watches the vault. The Node process remains a small loopback-only static-file server for the generated output.

The site includes canonical URLs, Open Graph title/type/URL/description metadata, `robots.txt`, and `sitemap.xml`. The `Impressum` and `Datenschutzerklärung` pages are placeholders until legally reviewed text is supplied.

## Prerequisites and setup

- Node.js `20.19.2` (the version in `.nvmrc`)
- npm 9 or newer
- For browser regression tests: Python 3 with `venv`, and Chromium at `/usr/bin/chromium`
- For the private preview: Tailscale installed, authenticated, and able to run `tailscale serve`
- For system service administration: an account with `sudo`

From the repository checkout, install the lockfile-resolved npm dependencies:

```sh
cd engineering-blog
npm ci
```

There are currently no third-party npm runtime dependencies; `npm ci` still establishes the lockfile-based local installation expected by the project.

## Build and view locally

Build the checked-in site source into `dist/`:

```sh
npm run build
```

Start a loopback-only development server after building. Stop it with `Ctrl-C` when finished:

```sh
npm run serve
```

Open <http://127.0.0.1:8080/> from the same machine. To use a different local port, set `PORT`; to bind another interface, set `HOST` deliberately:

```sh
PORT=8081 npm run serve
```

`npm run serve` always rebuilds first. `npm run build` deletes and recreates the output directory, then copies `src/site/` to `dist/`. `dist/` is generated and ignored by Git; edit `src/site/`, never `dist/`.

## Site content and structure

```text
src/site/                         Source HTML and static assets
src/site/index.html               Homepage and the two post cards/excerpts
src/site/<post-slug>/index.html   Individual mock post pages
src/site/impressum/index.html     Legal-page placeholder
src/site/datenschutzerklaerung/   Data-protection-page placeholder
src/site/assets/                  Checked-in CSS, fonts, logo, and illustrations
src/site/robots.txt               Crawler policy
src/site/sitemap.xml              Public canonical URL list
dist/                             Generated static site (ignored)
scripts/                          Build and local-server programs
deploy/                           systemd, Tailscale, and Cloudflare templates
tests/                            Static checks and Robot Framework regression suite
```

The two mock posts are:

- `src/site/a-small-deployment-pipeline-is-still-a-system/index.html`
- `src/site/what-i-want-from-a-local-first-engineering-blog/index.html`

Edit their HTML directly for the MVP. If a title, date, URL, or excerpt changes, also update the matching card in `src/site/index.html`, the affected canonical and Open Graph metadata, and `src/site/sitemap.xml`. The homepage currently lists the newer post first and renders exactly five excerpt lines per card. Keep all assets local and preserve the footer links to both legal pages.

The Go parser recognizes the preview-post metadata format; generated pages, redirects, and local preview output are handled automatically. There is no post-publishing command yet. `deploy/wrangler.example.jsonc` is only an unconfigured Workers Static Assets template; it is not a deploy command or a Cloudflare configuration to copy into production.

## Local systemd service

The deployment uses two units: `engineering-blog-preview.service` transforms and watches `/root/obsidian-vault/Engineering Blog` as root because the vault is private, while `engineering-blog.service` serves `/var/lib/engineering-blog/site` as the dedicated unprivileged `engineering-blog` account on `127.0.0.1:8080`. The installer compiles the versioned Go binary under `/opt/engineering-blog`, installs both units, and makes only the generated state directory writable at runtime.

Install or update the versioned unit, then enable and start it:

```sh
sudo deploy/install-service.sh
```

Inspect the running service and its recent log entries:

```sh
systemctl status engineering-blog-preview.service engineering-blog.service --no-pager
journalctl -u engineering-blog-preview.service -n 100 --no-pager
```

Follow logs while reproducing a problem with:

```sh
journalctl -fu engineering-blog-preview.service
```

If the service will not start, first check that `/opt/engineering-blog/src/site` exists, that Go is installed, and that the vault remains readable to the root-owned transformer. After changing source assets or pipeline code, reinstall the versioned runtime files and restart both units:

```sh
sudo deploy/install-service.sh
```

Then verify its local response:

```sh
curl --fail --show-error http://127.0.0.1:8080/
```

## Private Tailscale preview

Tailscale Serve terminates HTTPS and proxies the private preview to the loopback service. The checked-in setup reserves port 8443 because the host may already have unrelated port-443 Serve routes. It uses `tailscale serve`, never public `tailscale funnel`.

Apply the idempotent configuration after the local service is running:

```sh
sudo deploy/configure-tailscale-serve.sh
```

Inspect the routes:

```sh
tailscale serve status
```

The output must show `tailnet only` and a port-8443 route that proxies to `http://127.0.0.1:8080`. On the configured MVP host, open <https://debian-4gb-fsn1-1.taild36c23.ts.net:8443/> from an authorized tailnet device. If it does not load, confirm `tailscaled.service`, `engineering-blog.service`, and `tailscale serve status` are healthy:

```sh
systemctl status tailscaled.service engineering-blog.service --no-pager
```

The Serve configuration is stored by `tailscaled`, so it persists across restarts of both services.

## Validation

Run the dependency-free static checks (this builds `dist/` first):

```sh
npm run check
```

`npm run check` verifies the copied output, page metadata, sitemap and robots policy, local assets, no analytics/cookies/remote asset references, responsive CSS hooks, the service unit, and the Tailscale setup script.

### Go service foundation

The Go service is configured through environment variables. The vault directory and generated preview output directory are required and must not overlap. This prevents generated output from being watched as vault input.

```sh
BLOG_VAULT_DIRECTORY=/root/obsidian-vault \
BLOG_PREVIEW_OUTPUT_DIRECTORY=/var/lib/engineering-blog/generated-preview \
go run ./cmd/vault-preview-service
```

To run one reconciliation without starting the watcher, use `go run ./cmd/vault-preview-service --build-once` with the same settings.

Optional settings are `BLOG_DEBOUNCE_INTERVAL` (default `500ms`), `BLOG_RECONCILIATION_INTERVAL` (default `5m`), and `BLOG_PREVIEW_SERVER_URL` (default `http://127.0.0.1:8080`). Durations use Go's duration syntax. Send `SIGTERM` or press `Ctrl-C` to stop the service.

Run its focused suites with:

```sh
go test ./...
go test -tags=integration ./...
```

The integration suite builds the service and confirms that it starts using valid local configuration, then shuts it down cleanly.

Run browser regression tests against the private preview:

```sh
npm run test:e2e
```

On first use, this creates the ignored `.venv/`, installs the pinned packages in `tests/e2e/requirements.txt`, and initializes Robot Framework Browser. It runs Chromium headlessly at the configured preview URL. Reports are written to ignored `e2e-results/`:

```text
e2e-results/report.html
e2e-results/log.html
e2e-results/output.xml
```

To target another authorized preview host or change browser settings, pass `TARGET_URL`, `BROWSER`, `BROWSER_EXECUTABLE`, or `HEADLESS` in the environment when invoking the same test command. `DESKTOP_WIDTH`, `DESKTOP_HEIGHT`, `MOBILE_WIDTH`, and `MOBILE_HEIGHT` are also configurable. The suite does not create vault content: before and after every test it checks that its reserved fixture path is absent.

## MVP troubleshooting checklist

- A static check fails: run `npm run build` again and edit files only under `src/site/`, not `dist/`.
- The local site does not respond: inspect `engineering-blog.service` and its journal, then verify `curl --fail --show-error http://127.0.0.1:8080/`.
- The HTTPS preview does not respond: verify that Tailscale is connected, the local service is active, and `tailscale serve status` shows the tailnet-only `:8443` proxy route.
- Robot tests cannot connect: ensure the test machine is authorized in the tailnet and that `TARGET_URL` matches the Serve URL; inspect `e2e-results/log.html` after a run.
- A browser test cannot start: check Python's `venv` support and `/usr/bin/chromium`, then rerun `npm run test:e2e` to install or initialize the local test tooling.

For the full intended product direction and the MVP boundaries, see the [specification](../obsidian-vault/Agent%20Memories/Engineering%20Blog%20Website/Spec.md). The implementation in this repository is authoritative for currently supported commands.
