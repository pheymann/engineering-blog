#!/bin/sh
set -eu

: "${BLOG_PUBLIC_URL:?Set BLOG_PUBLIC_URL to the HTTPS GitHub Pages URL to run live verification.}"
public_url=${BLOG_PUBLIC_URL%/}

case "$public_url" in
	https://*) ;;
	*) echo "BLOG_PUBLIC_URL must start with https://" >&2; exit 2 ;;
esac

curl --fail --location --show-error --head "$public_url/"
curl --fail --show-error "$public_url/" | grep -F "${public_url}/"
curl --fail --location --show-error --head "$public_url/assets/styles.css"
curl --fail --location --show-error --head "$public_url/robots.txt"
curl --fail --location --show-error --head "$public_url/sitemap.xml"

http_url="http://${public_url#https://}"
effective_url=$(curl --fail --location --silent --show-error --output /dev/null --write-out '%{url_effective}' "$http_url/")
case "$effective_url" in
	"$public_url"/*) ;;
	*) echo "HTTP did not redirect to $public_url: $effective_url" >&2; exit 1 ;;
esac

echo "GitHub Pages public verification passed for $public_url"
