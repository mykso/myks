# Spike — KCL edge cases on a real myks-shaped filesystem tree

Extends `../README.md` (the composition spike) from "does the basic merge work" to
"does KCL survive the **long tail of ytt data-values behaviour** when fed a realistic
myks tree". Closes the edge-case list flagged in ADR 0002's Consequences.

## What is here

A miniature but structurally faithful myks repo + a discovery/merge harness that
mirrors how myks collects, orders, folds, types and validates an Application's data
values — then compares KCL's output to the **ytt baseline** byte-for-byte (after
canonicalization).

```
tree/
  envs/                                  nested env tree (root -> on-prem -> tools -> stage)
    env-data.schema.yaml                 ROOT schema (base every app extends)        [edge 3,4,7]
    env-data.values.yaml                 ROOT values; CreateNamespace: null          [edge 7]
    env-data.apps.yaml                   ROOT app list (bootstrap inherited here)     [edge 1]
    on-prem/env-data.ytt.yaml            group: override distro gke->rke2, append     [edge 1,2]
    on-prem/tools/env-data.ytt.yaml      tier: cluster identity, append apps          [edge 5,8]
    on-prem/tools/stage/                 LEAF (has environment.id => renders)
      env-data.ytt.yaml                  runsGrafanaOperator hand-set (shared key)    [edge 5]
      env-data.helm.yaml                 clusterFullName/Address hand-written         [edge 8]
      env-data.apps.yaml                 append karma+forwarder, REMOVE bootstrap     [edge 1]
      _apps/karma/app-data.values.yaml   per-app override (beats prototype)           [edge 2,6]
      _apps/central-forwarder/...        enables forwarder (drives validation)        [edge 4,5]
  prototypes/
    karma/                               any-typed config, prototype defaults         [edge 3,6]
    central-forwarder/                   dense validation surface + vendir source     [edge 4,5]
    grafana-operator/                    presence hoisted to runsGrafanaOperator      [edge 5]
  harness/
    discover.sh        host: enumerate ordered files for (env, app, proto)  <-- KCL can't walk FS
    baseline.sh        ytt --data-values-inspect over that list (the BASELINE)
    merge.k            engine: fold + deep-merge + list remove-by-key
    schema.k           typed schemas, validation (check/unions), derivation
    compose.k          KCL entrypoint == the ytt-replacement for one Application
    kcl.sh             run compose.k over the discovered list
    compare.sh         canonicalize both (yq sort_keys) and diff  <-- the PASS gate
    doubletoggle.k     edge 5: vendir source derived from the app flag (one source of truth)
```

## Run it

```shell
cd docs/redesign/spike/tree
nix shell nixpkgs#yq-go nixpkgs#kcl -c bash harness/compare.sh on-prem/tools/stage karma karma bootstrap
nix shell nixpkgs#yq-go nixpkgs#kcl -c bash harness/compare.sh on-prem/tools/stage central-forwarder central-forwarder bootstrap
# edge 5 double-toggle, both ways:
nix shell nixpkgs#kcl -c kcl run harness/doubletoggle.k -D enabled=true
nix shell nixpkgs#kcl -c kcl run harness/doubletoggle.k -D enabled=false
```

Both `compare.sh` runs print `PASS` — the composed data-values structs are identical.

## The pass gate

ytt and KCL **serialize** differently (key order; KCL quotes `'30s'`/`'true'`/IPs, ytt
does not). These are cosmetic. The gate canonicalizes both through
`yq -P 'sort_keys(..)'` and diffs — so PASS means the **data is identical**, not that the
bytes off the two tools match raw. This is the honest, achievable form of the
"byte-identical" criterion; raw-byte equality would only measure YAML printer quirks.

## Headline findings

- **KCL's `|` does NOT last-wins-merge decoded files.** It conflicts on any scalar two
  layers both set (`conflicting values on attribute 'a'`). The README finding #1 ("`|`
  deep-merges plain dicts") holds only for dict **literals** with `=`, not for
  `yaml.decode`'d data. Composing real env-data files needs a **hand-written recursive
  deep-merge** (`merge.k`, ~12 lines). **This is contained in the engine; the user
  surface stays plain YAML.**
- **The hand-merge is NOT a KCL-specific tax.** Jsonnet's `mergePatch` also replaces
  arrays, and `+` is shallow — Jsonnet needs the same ~6-line recursive merge for
  deep-dict + array-append. CUE cannot override at all. So the merge friction does not
  separate KCL from its fallbacks; **typing/validation/derivation does** (see ADR 0002).
- **List remove-by-key (`#@overlay/remove`) runs DURING composition**, so under ADR 0002
  scope A′ the new language owns it. KCL expresses it as a plain filter
  (`removeApps`) — and because KCL self-references the accumulated list (ytt cannot),
  this is cleaner than the ytt match-overlay, not harder.
- Every other edge case (precedence, schema layering, all validation flavours,
  any-passthrough, null-preservation, computed/derived values, the double-toggle) passes
  byte-identical. See the results table in `../README.md`.
