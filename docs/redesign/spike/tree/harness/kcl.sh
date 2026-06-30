#!/usr/bin/env bash
# KCL composition: the new-language equivalent of baseline.sh.
# Usage: kcl.sh <leaf-rel-path> <app> <proto> [removeProtos-csv]
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
PATHS=$("$HERE/discover.sh" "$1" "$2" "$3" --values | paste -sd, -)
exec kcl run "$HERE/compose.k" "$HERE/schema.k" "$HERE/merge.k" \
  -D paths="$PATHS" -D proto="$3" -D removeProtos="${4:-}"
