# Paul's Engineering Blog

This repository is the MVP for Paul's Engineering Blog. It is a static, privacy-focused preview of `engineering.paulheymann.de`: plain HTML and CSS, local fonts and images, no JavaScript, cookies, analytics, or external runtime downloads.

The repository contains the responsive site design, local vault-to-HTML preview pipeline, private Tailscale HTTPS preview, GitHub Pages production publishing, and automated checks.

The Go vault-preview service parses and validates tagged Markdown, builds the local preview and production `/docs` output, publishes changed `/docs` commits, and watches the vault. The Node process remains a small loopback-only static-file server for the generated output.

The site includes canonical URLs, Open Graph title/type/URL/description metadata, `robots.txt`, and `sitemap.xml`. The `Impressum` and `Datenschutzerklärung` pages contain the current supplied text and should receive legal review before relying on them as legal advice.

## Prerequisites and setup

- Node.js `20.19.2` (the version in `.nvmrc`)
- npm 9 or newer
- Chromium at `/usr/bin/chromium` for build-time Mermaid diagram rendering and browser regression tests
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
src/site/index.html               Homepage shell used when no vault posts qualify
src/site/impressum/index.html     Legal-page placeholder
src/site/datenschutzerklaerung/   Data-protection-page placeholder
src/site/assets/                  Checked-in CSS, fonts, logo, and illustrations
src/site/robots.txt               Crawler policy
src/site/sitemap.xml              Public canonical URL list
dist/                             Generated static site (ignored)
scripts/                          Build and local-server programs
deploy/                           systemd, Tailscale, and GitHub Pages documentation
tests/                            Static checks and Robot Framework regression suite
```

No mock posts are checked into the site. The Go parser recognizes the preview-post metadata format; generated pages, redirects, and local preview output are handled automatically. Keep all assets local and preserve the footer links to both legal pages. GitHub Pages owner setup and live verification are documented in [`deploy/github-pages.md`](deploy/github-pages.md).

Fenced `mermaid` code blocks are rendered to deterministic SVG assets during preview and production builds. The pinned local Mermaid CLI uses `/usr/bin/chromium`; readers receive only static HTML and SVG, with no client-side Mermaid JavaScript or external runtime download. Invalid or unterminated Mermaid blocks fail the build without replacing the last valid site.

## Local systemd service

The deployment uses two units: `engineering-blog-preview.service` transforms and watches `/root/obsidian-vault/Engineering Blog` as root because the vault is private, while `engineering-blog.service` serves `/var/lib/engineering-blog/site` as the dedicated unprivileged `engineering-blog` account on `127.0.0.1:8080`. Mermaid rendering is executed as the separate unprivileged `engineering-blog-mermaid` account, so the transformer unit deliberately omits `NoNewPrivileges` and `RestrictSUIDSGID`; either directive would prevent its exec-time credential drop. The remaining filesystem, kernel, private-temporary-directory, and address-family protections stay enabled. The installer creates a Git checkout under `/opt/engineering-blog` so the transformer can commit only changed `docs/` output; its production build state is durable in `/var/lib/engineering-blog/production-state.json`, never in the replaceable checkout. Before replacement, the installer stops the publisher and refuses to proceed if the existing checkout has unpushed commits or uncommitted changes.

Before enabling publishing, install a root-only Git configuration and SSH credential outside the checkout. Use [`deploy/git-publisher.env.example`](deploy/git-publisher.env.example) as the environment-file shape, configure `user.name` and `user.email` in its referenced Git config, and point its SSH config at a root-only deploy key with write access to `pheymann/engineering-blog`. Do not put a token or private key in the repository, unit file, or journal-visible command line.

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

`BLOG_PRODUCTION_OUTPUT_DIRECTORY` defaults to `docs` and `BLOG_GIT_REPOSITORY_DIRECTORY` defaults to the current working directory. On a successful changed production build, the service stages only `docs`, creates the deterministic `Publish generated site` commit, and pushes `HEAD` to `origin/main`. An unchanged `docs` tree performs no new commit or push; it only retries a previous docs-only publishing commit that is not yet on `origin/main`. Git identity, dirty-index/commit, authentication, and push errors are reported to the service journal; a failed push leaves its local `docs`-only commit available for that later retry.

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

To target another authorized preview host or change browser settings, pass `TARGET_URL`, `BROWSER`, `BROWSER_EXECUTABLE`, or `HEADLESS` in the environment when invoking the same test command. `DESKTOP_WIDTH`, `DESKTOP_HEIGHT`, `MOBILE_WIDTH`, and `MOBILE_HEIGHT` are also configurable. Pipeline cases create and remove their reserved fixture directory in the live preview vault; the isolated publication suite below never accesses that vault.

Run the production publication workflow separately:

```sh
npm run test:publish
```

This Robot test runs a Go integration test with a temporary vault, cloned checkout, and local bare Git remote. It covers initial publication, update, no-op reconciliation, and the `#publish` removal freeze without reading the real vault or contacting `origin`.

After the owner enables Pages and DNS, run the deliberate live probe:

```sh
BLOG_PUBLIC_URL=https://engineering.paulheymann.de npm run verify:public
```

It checks HTTPS, HTTP-to-HTTPS redirect behavior, homepage metadata, styles, `robots.txt`, and `sitemap.xml`; it never publishes content. See [`deploy/github-pages.md`](deploy/github-pages.md) for the required owner setup, recovery, and credential-rotation procedures.

## MVP troubleshooting checklist

- A static check fails: run `npm run build` again and edit files only under `src/site/`, not `dist/`.
- The local site does not respond: inspect `engineering-blog.service` and its journal, then verify `curl --fail --show-error http://127.0.0.1:8080/`.
- The HTTPS preview does not respond: verify that Tailscale is connected, the local service is active, and `tailscale serve status` shows the tailnet-only `:8443` proxy route.
- Robot tests cannot connect: ensure the test machine is authorized in the tailnet and that `TARGET_URL` matches the Serve URL; inspect `e2e-results/log.html` after a run.
- A browser test cannot start: check Python's `venv` support and `/usr/bin/chromium`, then rerun `npm run test:e2e` to install or initialize the local test tooling.
- A production push fails: inspect `journalctl -u engineering-blog-preview.service -n 100 --no-pager`; do not amend, reset, or force-push the retained publication commit. Follow the failed-push recovery steps in [`deploy/github-pages.md`](deploy/github-pages.md).

For the full intended product direction and the MVP boundaries, see the [specification](../obsidian-vault/Agent%20Memories/Engineering%20Blog%20Website/Project%20-%20Build%20a%20blog%20website.md). The implementation in this repository is authoritative for currently supported commands.
