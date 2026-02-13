#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   ./scripts/run-with-env.sh [path/to/envfile]

ENV_FILE="${1:-./deploy/sync-lite.env.example}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "env file not found: $ENV_FILE" >&2
  exit 1
fi

set -a
# shellcheck source=/dev/null
source "$ENV_FILE"
set +a

exec go run .
