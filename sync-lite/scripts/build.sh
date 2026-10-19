#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."
GO="${GO:-go}"
BACKEND="${1:-modernc}"
export GOWORK=off
export GOOS="${GOOS:-linux}"
export GOARCH="${GOARCH:-amd64}"

if [[ "$GOOS" != linux || ! "$GOARCH" =~ ^[a-z0-9]+$ ]]; then
  echo 'This helper builds Linux targets; GOARCH must be a Go architecture name.' >&2
  exit 1
fi
case "$BACKEND" in
  modernc)
    export CGO_ENABLED=0
    SUFFIX=""
    BUILD_TAGS=()
    ;;
  mattn)
    export CGO_ENABLED=1
    SUFFIX="-cgo"
    BUILD_TAGS=(-tags sqlite_cgo)
    ;;
  *)
    echo 'Usage: ./scripts/build.sh [modernc|mattn]' >&2
    exit 1
    ;;
esac

if [[ -L dist ]]; then
  echo 'Refusing to build into a symlinked dist directory.' >&2
  exit 1
fi
mkdir -p dist
OUTPUT="dist/sync-lite-$GOOS-$GOARCH$SUFFIX"
for artifact in "$OUTPUT" "$OUTPUT.sha256" "$OUTPUT.build-info.txt"; do
  if [[ -L "$artifact" ]]; then
    echo "Refusing to overwrite a symlinked build artifact: $artifact" >&2
    exit 1
  fi
done
TEMP_DIR="$(mktemp -d dist/.build.XXXXXXXX)"
trap 'rm -rf -- "$TEMP_DIR"' EXIT
"$GO" build -trimpath "${BUILD_TAGS[@]}" -o "$TEMP_DIR/sync-lite" .
if [[ "$GOOS" == "$("$GO" env GOHOSTOS)" && "$GOARCH" == "$("$GO" env GOHOSTARCH)" ]]; then
  "$TEMP_DIR/sync-lite" --version
fi
mv -- "$TEMP_DIR/sync-lite" "$OUTPUT"
"$GO" version -m "$OUTPUT" >"$OUTPUT.build-info.txt"
sha256sum "$OUTPUT" >"$OUTPUT.sha256"
printf 'Built %s\n' "$OUTPUT"
