#!/usr/bin/env bash
# ytt BASELINE: compose the merged data-values struct exactly as myks does.
# Usage: baseline.sh <leaf-rel-path> <app> <proto>
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
args=()
while IFS= read -r f; do args+=(-f "$f"); done < <("$HERE/discover.sh" "$1" "$2" "$3" --all)
exec ytt "${args[@]}" --data-values-inspect
