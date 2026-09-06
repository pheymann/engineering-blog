#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
application_directory=${ENGINEERING_BLOG_APPLICATION_DIRECTORY:-/opt/engineering-blog}
state_directory=${ENGINEERING_BLOG_STATE_DIRECTORY:-/var/lib/engineering-blog}
unit_directory=${ENGINEERING_BLOG_UNIT_DIRECTORY:-/etc/systemd/system}
systemctl_command=${ENGINEERING_BLOG_SYSTEMCTL:-systemctl}
service_user=engineering-blog
legacy_state_file=.engineering-blog-production-state.json
service_was_active=false

restore_publisher_on_failure() {
	status=$?
	trap - EXIT
	if [ "$status" -ne 0 ] && [ "$service_was_active" = true ]; then
		"$systemctl_command" restart engineering-blog-preview.service >/dev/null 2>&1 || true
	fi
	exit "$status"
}

is_known_legacy_layout() {
	for required in README.md bin cmd go.mod go.sum internal package-lock.json package.json scripts src; do
		[ -e "$application_directory/$required" ] || return 1
	done
	find "$application_directory" -mindepth 1 -maxdepth 1 \
		! -name README.md ! -name bin ! -name cmd ! -name go.mod ! -name go.sum \
		! -name internal ! -name package-lock.json ! -name package.json ! -name scripts \
		! -name src ! -name "$legacy_state_file" -print -quit | grep -q . && return 1
	return 0
}

if ! id "$service_user" >/dev/null 2>&1; then
	useradd --system --user-group --home-dir /nonexistent --shell /usr/sbin/nologin "$service_user"
fi

install -d -o root -g "$service_user" -m 0750 "$application_directory"
if ! git -C "$project_root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	echo "deployment source must be a Git checkout" >&2
	exit 1
fi

# Stop the publisher before examining its checkout so a failed-push commit
# cannot appear between this safety check and replacement. A failure after this
# point restores a service that had been active before installation started.
if "$systemctl_command" is-active --quiet engineering-blog-preview.service; then
	service_was_active=true
fi
trap restore_publisher_on_failure EXIT
"$systemctl_command" stop engineering-blog-preview.service 2>/dev/null || true

if [ -e "$application_directory/.git" ]; then
	if ! git -C "$application_directory" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		echo "refusing to replace non-Git application directory $application_directory" >&2
		exit 1
	fi
	if ! git -C "$application_directory" fetch origin main; then
		echo "refusing to replace checkout: cannot verify origin/main" >&2
		exit 1
	fi
	if ! git -C "$application_directory" merge-base --is-ancestor HEAD FETCH_HEAD; then
		echo "refusing to replace checkout: local commits are not on origin/main" >&2
		exit 1
	fi
	if [ -n "$(git -C "$application_directory" status --porcelain)" ]; then
		echo "refusing to replace checkout with uncommitted changes" >&2
		exit 1
	fi
elif [ -n "$(find "$application_directory" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
	if ! is_known_legacy_layout; then
		echo "refusing to replace non-Git application directory $application_directory" >&2
		exit 1
	fi
	if [ -f "$application_directory/$legacy_state_file" ]; then
		install -d -o root -g "$service_user" -m 0750 "$state_directory"
		install -o root -g "$service_user" -m 0640 "$application_directory/$legacy_state_file" "$state_directory/production-state.json"
	fi
fi

rm -rf "$application_directory"
git clone --no-local "$project_root" "$application_directory"
git -C "$application_directory" remote set-url origin "$(git -C "$project_root" remote get-url origin)"
mkdir -p "$application_directory/bin"
(cd "$application_directory" && go build -o bin/vault-preview-service ./cmd/vault-preview-service)
chown -R root:"$service_user" "$application_directory"
find "$application_directory" -type d -exec chmod 0750 {} +
find "$application_directory" -type f -exec chmod 0640 {} +
chmod 0755 "$application_directory/bin/vault-preview-service"

install -d -o root -g root -m 0755 "$unit_directory"
install -o root -g root -m 0644 "$project_root/deploy/engineering-blog.service" "$unit_directory/engineering-blog.service"
install -o root -g root -m 0644 "$project_root/deploy/engineering-blog-preview.service" "$unit_directory/engineering-blog-preview.service"
"$systemctl_command" daemon-reload
"$systemctl_command" enable engineering-blog-preview.service
"$systemctl_command" enable engineering-blog.service
"$systemctl_command" restart engineering-blog-preview.service
"$systemctl_command" restart engineering-blog.service
trap - EXIT
