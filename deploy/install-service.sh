#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
application_directory=/opt/engineering-blog
service_user=engineering-blog

if ! id "$service_user" >/dev/null 2>&1; then
  useradd --system --user-group --home-dir /nonexistent --shell /usr/sbin/nologin "$service_user"
fi

install -d -o root -g "$service_user" -m 0750 "$application_directory"
find "$application_directory" -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
tar -C "$project_root" \
	--exclude=.git --exclude=node_modules --exclude=dist \
	-cf - README.md package.json package-lock.json go.mod go.sum cmd internal scripts src \
	| tar -C "$application_directory" --no-same-owner -xf -
mkdir -p "$application_directory/bin"
(cd "$application_directory" && go build -o bin/vault-preview-service ./cmd/vault-preview-service)
chown -R root:"$service_user" "$application_directory"
find "$application_directory" -type d -exec chmod 0750 {} +
find "$application_directory" -type f -exec chmod 0640 {} +
chmod 0755 "$application_directory/bin/vault-preview-service"

install -o root -g root -m 0644 "$project_root/deploy/engineering-blog.service" /etc/systemd/system/engineering-blog.service
install -o root -g root -m 0644 "$project_root/deploy/engineering-blog-preview.service" /etc/systemd/system/engineering-blog-preview.service
systemctl daemon-reload
systemctl enable engineering-blog-preview.service
systemctl enable engineering-blog.service
systemctl restart engineering-blog-preview.service
systemctl restart engineering-blog.service
