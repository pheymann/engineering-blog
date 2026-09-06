# GitHub Pages and custom-domain setup

This repository publishes the generated `docs/` directory on `main` to the
production hostname `engineering.paulheymann.de`. The site source contains a
`CNAME` file with that hostname; the production builder copies it into `docs/`.

## One-time owner setup

An administrator of `pheymann/engineering-blog` must perform these steps after
the first production output has been pushed to `main`:

1. In GitHub, open **Settings** > **Pages** for `pheymann/engineering-blog`.
2. Under **Build and deployment**, select **Deploy from a branch**, then choose
   branch `main` and folder `/docs`; save the setting.
3. Under **Custom domain**, enter `engineering.paulheymann.de` and save it.
4. At GoDaddy, add exactly this record to the existing `paulheymann.de` zone:

   | Type | Name | Value |
   | --- | --- | --- |
   | CNAME | `engineering` | `pheymann.github.io` |

   Do not point the subdomain at `paulheymann.de` or at a repository path, and
   do not alter the apex, mail, nameserver, or any unrelated DNS record.
5. Wait for the DNS check beside the custom domain in GitHub Pages to succeed.
   Once GitHub has issued its certificate, enable **Enforce HTTPS**. The option
   can take up to 24 hours to become available.

GitHub recommends additionally verifying a domain in the account settings with
a TXT record. That security-hardening step is intentionally not part of this
site's required single GoDaddy CNAME change; perform it only if the domain
owner authorizes an extra DNS record.

## Verification

Run these checks after DNS propagation and GitHub Pages deployment complete:

```sh
curl --fail --location --head https://engineering.paulheymann.de/
curl --fail https://engineering.paulheymann.de/ | grep -F 'https://engineering.paulheymann.de/'
curl --fail --head https://engineering.paulheymann.de/assets/styles.css
dig +short CNAME engineering.paulheymann.de
```

The final `dig` command must report `pheymann.github.io.` (possibly followed by
GitHub's resolver chain). The HTTPS response must be successful, and an HTTP
request must redirect to HTTPS. The deployed homepage and generated posts must
retain canonical links, Open Graph URLs, `robots.txt`, and `sitemap.xml` using
`https://engineering.paulheymann.de`.

Current GitHub guidance: [custom subdomain setup](https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/managing-a-custom-domain-for-your-github-pages-site#configuring-a-subdomain) and [HTTPS enforcement](https://docs.github.com/en/pages/getting-started-with-github-pages/securing-your-github-pages-site-with-https).

## Normal publishing and recovery

The root-owned `engineering-blog-preview.service` produces `docs/` whenever a
valid vault file has `#blog #engineering #publish`. A changed tree is committed
as `Publish generated site` and pushed to `origin/main`; GitHub Pages then
deploys it. A file that loses `#publish` is frozen at its last public output.

Check the most recent run and publication commit with:

```sh
journalctl -u engineering-blog-preview.service -n 100 --no-pager
git -C /opt/engineering-blog log --oneline -5
git -C /opt/engineering-blog status --short
```

For failed-push recovery, correct only the reported authentication, remote, or
network problem and let the next reconciliation run (or restart the service)
retry. The service recognizes its unpushed docs-only `Publish generated site`
commit and pushes that exact commit; it does not create another commit for an
unchanged tree. Do not use `git reset`, amend the commit, force-push, or stage
unrelated work to recover it. Escalate a non-fast-forward conflict before any
history-changing action.

## Credentials and rotation

Publishing credentials live outside the repository. Install
`/etc/engineering-blog/git-publisher.env` with mode `0600`, set its
`GIT_CONFIG_GLOBAL` to a root-only Git config containing the publishing
identity, and set `GIT_SSH_COMMAND` to an SSH config that references a root-only
write deploy key. Neither a token nor a private key belongs in the checkout,
unit file, shell command line, or journal.

To rotate the credential, add the replacement key in GitHub first, update the
root-only key and SSH configuration, run a harmless authenticated Git command
as root from `/opt/engineering-blog`, then remove the old key in GitHub. Restart
`engineering-blog-preview.service` and inspect its journal. This credential
rotation sequence keeps the old key available until the replacement has been
verified.

## Opt-in public verification

After Pages, DNS, and HTTPS are active, run this repository script from a
trusted machine:

```sh
BLOG_PUBLIC_URL=https://engineering.paulheymann.de npm run verify:public
```

It performs read-only HTTP checks for the homepage, canonical production URL,
CSS, `robots.txt`, `sitemap.xml`, and HTTP-to-HTTPS redirect. It is opt-in:
without `BLOG_PUBLIC_URL` it exits before making a network request.
