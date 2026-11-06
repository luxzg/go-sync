#!/usr/bin/env bash
# Fixed, local release preparation only; never installs, deploys or publishes.
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$SCRIPT_DIR/release_artifacts.py" "$@"
