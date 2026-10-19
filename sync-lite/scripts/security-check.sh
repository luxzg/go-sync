#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."
GOVULNCHECK="${GOVULNCHECK:-govulncheck}"
export GOWORK=off

if ! command -v "$GOVULNCHECK" >/dev/null; then
  echo 'govulncheck is required; see DEVELOPMENT_SETUP.md for installation.' >&2
  exit 1
fi
"$GOVULNCHECK" -version
CGO_ENABLED=0 "$GOVULNCHECK" ./...
CGO_ENABLED=1 "$GOVULNCHECK" -tags sqlite_cgo ./...
