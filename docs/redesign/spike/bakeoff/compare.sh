#!/usr/bin/env bash
# Correctness gate: run a candidate's driver, canonicalize both sides, diff vs golden.
#   nix shell nixpkgs#kcl nixpkgs#yq-go -c bash compare.sh 'kcl run kcl/'
set -euo pipefail
cd "$(dirname "$0")"
EMIT="${1:-kcl run kcl/}"

canon() { yq -P 'sort_keys(..) | ... comments=""'; }

eval "$EMIT" | canon > /tmp/bakeoff.candidate.yaml
canon < fixtures/golden.yaml > /tmp/bakeoff.golden.yaml

if diff -u /tmp/bakeoff.golden.yaml /tmp/bakeoff.candidate.yaml; then
  echo "PASS — resolved tree matches golden"
else
  echo "FAIL — see diff above"; exit 1
fi
