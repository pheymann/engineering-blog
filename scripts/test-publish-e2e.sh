#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
venv="$project_root/.venv"

if [ ! -x "$venv/bin/robot" ]; then
	echo "Robot Framework is not installed; run npm run test:e2e once first." >&2
	exit 1
fi

exec "$venv/bin/robot" \
	--outputdir "$project_root/e2e-results/publishing" \
	"$project_root/tests/publishing.robot"
