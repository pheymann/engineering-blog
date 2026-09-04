#!/usr/bin/env bash
set -euo pipefail

# Use a separate HTTPS port so the blog owns `/`; existing Serve routes on 443
# continue to serve their current applications.
tailscale serve --https=8443 --bg http://127.0.0.1:8080
