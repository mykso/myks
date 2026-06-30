#!/usr/bin/env bash
# Host-side file DISCOVERY — the part KCL cannot do itself (it is not a filesystem
# walker). Given a leaf env path + application + prototype, emit the ordered file list
# myks would feed the composer: schemas first, then data values root->leaf, then
# prototype defaults, then per-app (_apps/<app>) overrides found at each tree level.
#
# Usage: discover.sh <leaf-rel-path> <app-name> <proto-name> [--schema|--values]
#   discover.sh on-prem/tools/stage karma karma --values
#
# Order = precedence (later wins). This list is the contract handed to BOTH backends:
#   ytt   -f <each>            (baseline)
#   kcl   compose.k -D paths=  (the same list, folded)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LEAF="$1"; APP="$2"; PROTO="$3"; MODE="${4:---values}"

# Build the chain of env directories root -> leaf.
envdirs=("$ROOT/envs")
acc="$ROOT/envs"
IFS='/' read -ra parts <<< "$LEAF"
for p in "${parts[@]}"; do acc="$acc/$p"; envdirs+=("$acc"); done

schemas=(); values=()

# 1. root env schema, 2. prototype schema (extends additively)
[[ -f "$ROOT/envs/env-data.schema.yaml" ]] && schemas+=("$ROOT/envs/env-data.schema.yaml")
[[ -f "$ROOT/prototypes/$PROTO/app-data.schema.yaml" ]] && schemas+=("$ROOT/prototypes/$PROTO/app-data.schema.yaml")

# 3. env-data.* down the tree (root -> leaf), then per-level _apps/<app> overrides
for d in "${envdirs[@]}"; do
  for suffix in values apps ytt helm; do
    f="$d/env-data.$suffix.yaml"
    [[ -f "$f" ]] && values+=("$f")
  done
done
# 4. prototype defaults (application-scope base)
[[ -f "$ROOT/prototypes/$PROTO/app-data.values.yaml" ]] && values+=("$ROOT/prototypes/$PROTO/app-data.values.yaml")
# 5. per-app overrides discovered at each level (deepest last = wins)
for d in "${envdirs[@]}"; do
  f="$d/_apps/$APP/app-data.values.yaml"
  [[ -f "$f" ]] && values+=("$f")
done

case "$MODE" in
  --schema) printf '%s\n' "${schemas[@]}" ;;
  --values) printf '%s\n' "${values[@]}" ;;
  --all)    printf '%s\n' "${schemas[@]}" "${values[@]}" ;;
esac
