#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
LITE_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
REPO_DIR="$(cd -- "$LITE_DIR/.." && pwd)"
GO="${GO:-go}"
BASELINE=1c0d957

command -v "$GO" >/dev/null
command -v git >/dev/null
command -v gcc >/dev/null
git -C "$REPO_DIR" cat-file -e "$BASELINE^{commit}"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sync-lite-upgrade.XXXXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
mkdir "$WORK_DIR/legacy"
# Archive an explicit source allowlist; never copy runtime databases or config.
git -C "$REPO_DIR" archive "$BASELINE" -- go.mod go.sum auth cache command datastore schema/protobuf sync-lite |
  tar -x -C "$WORK_DIR/legacy"
cp "$LITE_DIR/upgrade_compatibility_test.go" "$WORK_DIR/legacy/sync-lite/"
mkdir -p "$WORK_DIR/legacy/sync-lite/testdata"
cp "$LITE_DIR/testdata/v0.2.sql" "$WORK_DIR/legacy/sync-lite/testdata/"

export GOWORK=off
unset SYNC_LITE_UPGRADE_TEST_DIR SYNC_LITE_UPGRADE_STAGE SYNC_LITE_UPGRADE_LEAVE_WAL
(
  cd "$WORK_DIR/legacy/sync-lite"
  CGO_ENABLED=0 "$GO" test -c -o "$WORK_DIR/old.test" .
)
cd "$LITE_DIR"
CGO_ENABLED=0 "$GO" test -c -o "$WORK_DIR/modernc.test" .
CGO_ENABLED=1 "$GO" test -tags sqlite_cgo -c -o "$WORK_DIR/mattn.test" .

for mode in clean wal; do
  mkdir "$WORK_DIR/$mode"
  printf 'sync-lite disposable upgrade fixture\n' >"$WORK_DIR/$mode/.synthetic-upgrade-test"
  export SYNC_LITE_UPGRADE_TEST_DIR="$WORK_DIR/$mode"
  export SYNC_LITE_UPGRADE_STAGE=seed
  if [[ "$mode" == wal ]]; then
    export SYNC_LITE_UPGRADE_LEAVE_WAL=1
  fi
  (
    cd "$WORK_DIR/legacy/sync-lite"
    "$WORK_DIR/old.test" -test.run '^TestUpgradeFixture$' -test.v
  )
  unset SYNC_LITE_UPGRADE_LEAVE_WAL
  if [[ "$mode" == wal && ! -s "$WORK_DIR/$mode/fixture.db-wal" ]]; then
    echo 'Expected committed v0.2 WAL data for the recovery test' >&2
    exit 1
  fi
  for backend in modernc mattn; do
    SYNC_LITE_UPGRADE_STAGE="$backend" "$WORK_DIR/$backend.test" -test.run '^TestUpgradeFixture$' -test.v
  done
  SYNC_LITE_UPGRADE_STAGE=old-version "$WORK_DIR/old.test" -test.run '^TestUpgradeFixture$' -test.v
  echo "PASS: v0.2 -> v0.3 modernc -> v0.3 mattn -> v0.2 ($mode database)"
done
