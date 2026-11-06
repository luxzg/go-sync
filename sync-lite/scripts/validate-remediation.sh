#!/usr/bin/env bash
# Fixed validation entry points; never accepts arbitrary commands or revisions.
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."
if [[ $# -eq 0 ]]; then
  echo 'Usage: ./scripts/validate-remediation.sh {check|smoke|upgrades|supplemental|build|security|format|A01..A15}'
  echo 'Uses installed Go 1.26.10; disposable synthetic state only, no deployment.'
  exit 0
fi
if [[ $# -ne 1 ]]; then
  echo 'Expected one validation mode' >&2
  exit 2
fi
export GO="$HOME/sdk/go1.26.10/bin/go"
PATH="$(dirname "$GO"):$HOME/go/bin:$PATH"
export PATH
if [[ ! -x "$GO" ]]; then
  echo 'Install Go 1.26.10 in ~/sdk/go1.26.10 or review this pinned helper.' >&2
  exit 1
fi
case "$1" in
  format) exec "$(dirname "$GO")/gofmt" -w ./*.go testdata/session_reconciliation_test.go ;;
  A01|A02|A03|A04|A05|A06|A07|A08|A09|A10|A11|A12|A13|A14|A15)
    export GOWORK=off
    CGO_ENABLED=0 "$GO" test -count=1 -run "^TestAudit$1" ./...
    CGO_ENABLED=1 "$GO" test -race -tags sqlite_cgo -count=1 -run "^TestAudit$1" ./...
    ;;
  check) exec ./scripts/check.sh ;;
  smoke) exec ./scripts/smoke-test.sh ;;
  supplemental) exec ./scripts/test-session-reconciliation.sh ;;
  build) exec ./scripts/build.sh ;;
  security) exec ./scripts/security-check.sh ;;
  upgrades)
    for baseline in 1c0d957 287bdec 07d147a; do
      SYNC_LITE_UPGRADE_BASELINE="$baseline" ./scripts/test-upgrade.sh
    done
    ;;
  *) echo 'Unknown validation mode; run without arguments for usage' >&2; exit 2 ;;
esac
