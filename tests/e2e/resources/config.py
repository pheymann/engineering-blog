"""Environment-backed settings for the private preview regression suite."""

import os


TARGET_URL = os.environ.get(
    "TARGET_URL", "https://debian-4gb-fsn1-1.taild36c23.ts.net:8443"
).rstrip("/")
BROWSER = os.environ.get("BROWSER", "chromium")
BROWSER_EXECUTABLE = os.environ.get("BROWSER_EXECUTABLE", "/usr/bin/chromium")
HEADLESS = os.environ.get("HEADLESS", "true").lower() in {"1", "true", "yes"}
DESKTOP_WIDTH = int(os.environ.get("DESKTOP_WIDTH", "1440"))
DESKTOP_HEIGHT = int(os.environ.get("DESKTOP_HEIGHT", "900"))
MOBILE_WIDTH = int(os.environ.get("MOBILE_WIDTH", "390"))
MOBILE_HEIGHT = int(os.environ.get("MOBILE_HEIGHT", "844"))
VAULT_TEST_FIXTURE = os.environ.get(
    "VAULT_TEST_FIXTURE", "/root/obsidian-vault/.e2e-engineering-blog-fixture.md"
)
