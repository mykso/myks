#!/usr/bin/env bash
# Byte-identical gate (after canonicalization). ytt and KCL serialize differently
# (key order, scalar quoting), so we canonicalize BOTH through `yq -P sort_keys(..)`
# and diff. A clean diff == the composed data-values STRUCTS are identical.
# Usage: compare.sh <leaf-rel-path> <app> <proto> [removeProtos-csv]
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
canon() { yq -P 'sort_keys(..)' ; }
b=$(bash "$HERE/baseline.sh" "$1" "$2" "$3" | canon)
k=$(nix shell nixpkgs#kcl -c bash "$HERE/kcl.sh" "$1" "$2" "$3" "${4:-}" | canon)
if diff <(echo "$b") <(echo "$k") >/dev/null; then
  echo "PASS  $2 ($3): KCL struct == ytt struct"
else
  echo "DIFF  $2 ($3):"
  diff <(echo "$b") <(echo "$k") || true
fi
