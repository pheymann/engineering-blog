#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

fake_systemctl="$fixture/systemctl"
log="$fixture/systemctl.log"
cat >"$fake_systemctl" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"$ENGINEERING_BLOG_SYSTEMCTL_LOG"
exit 0
EOF
chmod 0700 "$fake_systemctl"

make_legacy() {
	directory=$1
	mkdir -p "$directory/bin" "$directory/cmd" "$directory/internal" "$directory/scripts" "$directory/src"
	touch "$directory/README.md" "$directory/go.mod" "$directory/go.sum" "$directory/package-lock.json" "$directory/package.json"
}

application="$fixture/legacy"
state="$fixture/state"
units="$fixture/units"
make_legacy "$application"
printf 'legacy production state\n' >"$application/.engineering-blog-production-state.json"
ENGINEERING_BLOG_APPLICATION_DIRECTORY="$application" \
ENGINEERING_BLOG_STATE_DIRECTORY="$state" \
ENGINEERING_BLOG_UNIT_DIRECTORY="$units" \
ENGINEERING_BLOG_SYSTEMCTL="$fake_systemctl" \
ENGINEERING_BLOG_SYSTEMCTL_LOG="$log" \
sh "$project_root/deploy/install-service.sh"

git -C "$application" rev-parse --is-inside-work-tree >/dev/null
git -C "$application" diff --quiet
git -C "$application" diff --cached --quiet
test -f "$state/production-state.json"
grep -Fx 'legacy production state' "$state/production-state.json" >/dev/null
grep -Fx 'stop engineering-blog-preview.service' "$log" >/dev/null
grep -Fx 'restart engineering-blog-preview.service' "$log" >/dev/null

unknown="$fixture/unknown"
mkdir -p "$unknown"
touch "$unknown/unrecognized"
: >"$log"
if ENGINEERING_BLOG_APPLICATION_DIRECTORY="$unknown" \
	ENGINEERING_BLOG_STATE_DIRECTORY="$fixture/other-state" \
	ENGINEERING_BLOG_UNIT_DIRECTORY="$fixture/other-units" \
	ENGINEERING_BLOG_SYSTEMCTL="$fake_systemctl" \
	ENGINEERING_BLOG_SYSTEMCTL_LOG="$log" \
	sh "$project_root/deploy/install-service.sh"; then
	echo "unknown non-Git application directory was unexpectedly replaced" >&2
	exit 1
fi
test -f "$unknown/unrecognized"
grep -Fx 'restart engineering-blog-preview.service' "$log" >/dev/null
