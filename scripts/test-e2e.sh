#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
venv="$project_root/.venv"

if [ ! -x "$venv/bin/python" ]; then
  python3 -m venv "$venv"
fi

"$venv/bin/python" -m pip install --disable-pip-version-check --requirement "$project_root/tests/e2e/requirements.txt"
browser_ready=$("$venv/bin/python" -c 'from pathlib import Path; import Browser; print(Path(Browser.__file__).parent / "wrapper/node_modules/playwright/package.json")')
if [ ! -f "$browser_ready" ]; then
  "$venv/bin/rfbrowser" init
fi

exec "$venv/bin/robot" \
  --outputdir "$project_root/e2e-results" \
  --variablefile "$project_root/tests/e2e/resources/config.py" \
  "$project_root/tests/e2e"
