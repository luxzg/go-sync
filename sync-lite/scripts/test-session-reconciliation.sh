#!/usr/bin/env bash
# Investigation helper v0.2: synthetic state only; never repairs an input DB.
set -euo pipefail

if (( $# != 0 )); then
  echo 'Usage: ./scripts/test-session-reconciliation.sh (no arguments; synthetic data only)' >&2
  exit 2
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
LITE_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
GO="${GO:-go}"
command -v "$GO" >/dev/null
command -v python3 >/dev/null
command -v gcc >/dev/null
if [[ -n "$(gofmt -l "$LITE_DIR/testdata/session_reconciliation_test.go")" ]]; then
  echo 'Format testdata/session_reconciliation_test.go with gofmt before running.' >&2
  exit 1
fi

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sync-lite-reconciliation.XXXXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
VIRTUAL_TEST="$LITE_DIR/sync_lite_reconciliation_experiment_test.go"
if [[ -e "$VIRTUAL_TEST" || -L "$VIRTUAL_TEST" ]]; then
  echo 'Refusing to overlay an existing file.' >&2
  exit 1
fi
# Inject investigation-only tests without adding a repair path to the service.
python3 - "$VIRTUAL_TEST" "$LITE_DIR/testdata/session_reconciliation_test.go" "$WORK_DIR/overlay.json" <<'PY'
import json
import sys

with open(sys.argv[3], "x", encoding="utf-8") as output:
    json.dump({"Replace": {sys.argv[1]: sys.argv[2]}}, output)
PY
cd "$LITE_DIR"
export GOWORK=off
CGO_ENABLED=0 "$GO" test -overlay "$WORK_DIR/overlay.json" -count=1 -run '^TestSessionReconciliationExperiment' -v .
CGO_ENABLED=1 "$GO" test -race -tags sqlite_cgo -overlay "$WORK_DIR/overlay.json" -count=1 -run '^TestSessionReconciliationExperiment' -v .
echo 'PASS: synthetic reconciliation experiment on both drivers; no real database was read or repaired'
