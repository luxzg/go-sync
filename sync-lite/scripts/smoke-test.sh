#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."
GO="${GO:-go}"
export GOWORK=off
command -v python3 >/dev/null
command -v openssl >/dev/null

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sync-lite-smoke.XXXXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
CGO_ENABLED=0 "$GO" build -trimpath -o "$WORK_DIR/modernc" .
CGO_ENABLED=1 "$GO" build -trimpath -tags sqlite_cgo -o "$WORK_DIR/mattn" .
python3 testdata/smoke_server.py "$WORK_DIR"
