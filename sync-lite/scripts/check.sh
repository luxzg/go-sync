#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."
GO="${GO:-go}"
export GOWORK=off

command -v "$GO" >/dev/null
command -v shellcheck >/dev/null
UNFORMATTED="$(gofmt -l ./*.go)"
if [[ -n "$UNFORMATTED" ]]; then
  printf 'Run gofmt on:\n%s\n' "$UNFORMATTED" >&2
  exit 1
fi
shellcheck scripts/*.sh
CGO_ENABLED=0 "$GO" vet ./...
CGO_ENABLED=1 "$GO" vet -tags sqlite_cgo ./...
CGO_ENABLED=0 "$GO" test -count=1 ./...
CGO_ENABLED=1 "$GO" test -race -count=1 ./...
CGO_ENABLED=1 "$GO" test -race -tags sqlite_cgo -count=1 ./...
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sync-lite-check.XXXXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
if CGO_ENABLED=0 "$GO" build -tags sqlite_cgo -o "$WORK_DIR/invalid" . >"$WORK_DIR/no-cgo.log" 2>&1; then
  echo 'sqlite_cgo unexpectedly built with CGO disabled' >&2
  exit 1
fi
if ! grep -q 'sqlite_cgo_requires_CGO_ENABLED_1_and_a_C_compiler' "$WORK_DIR/no-cgo.log"; then
  cat "$WORK_DIR/no-cgo.log" >&2
  exit 1
fi
echo 'PASS: formatting, ShellCheck, vet, default no-CGO tests, both race-enabled suites, and invalid-CGO guard'
