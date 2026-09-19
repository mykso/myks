---
status: accepted
---

# KCL engine integration: in-process eval, tree-as-discovery, per-repo mode, ytt bridge

How the KCL configuration layer (ADR 0002/0003/0004) plugs into the Go engine (ADR 0005). Five
coupled decisions, recorded together because they define one seam. Implementation detail lives in
`docs/redesign/design.md`.

## 1. KCL runs in-process via kcl-go

The engine evaluates the tree with `kcl-lang.io/kcl-go` (`kcl.Run` on the repo's config root), not
by exec-ing an external `kcl` binary. Matches the embedded-tool philosophy (one binary, no version
skew — the pain embedding ytt solved), and the language version is pinned by the Go dependency.
Rejected: external binary (user-managed version skew), embed-and-exec (`myks kcl …` — no benefit
over in-process for a library call).

## 2. The frozen resolved tree is the sole discovery mechanism

One evaluation returns the whole resolved tree (ADR 0003); the engine derives **everything** from
it — environment list, per-env app list, every app's resolved sync/render/ArgoCD config. No
filesystem walk for discovery: the user's in-language imports (ADR 0004) are the wiring, and the
tree output is the contract. Consequence: the tree schema must keep each app's resolved config a
self-contained, diffable unit — that is what lets tree-diff smart mode bolt on later without a
schema break.

## 3. Mode detection is per-repo

A repo is either legacy (ytt data-values) or KCL — detected by the KCL tree marker (`kcl.mod` at
the config root). No per-environment mixed mode: it would double the test matrix and smart-mode
complexity to buy a transition convenience the migration gate already provides (a repo converts
wholesale, validated byte-identical, then flips).

## 4. ytt keeps its data-values input API (the bridge)

ytt templates and overlays are retained (ADR 0002) and today consume data-values. In KCL mode the
engine feeds each app's resolved config to the ytt step as a **generated data-values file**.
Existing templates keep `#@ data.values.…` unchanged: data-values *authoring* is what dies;
data-values as ytt's input interface stays. This keeps the byte-identical migration gate reachable
with zero template edits.

## 5. myks schemas ship as an OCI KCL package

User trees import myks-provided schemas as a normal KCL dependency (`kcl mod add`, pinned in
`kcl.mod`, resolved by the LSP). The engine asserts the schema version found in the evaluated tree
against its supported range. Rejected: scaffolding schemas into user repos (drifts), generating
them on the fly (fights the LSP).

## Migration corollaries

- **Gate:** byte-identical rendered manifests between legacy and KCL mode, per app; waived only
  where derivations intentionally change values.
- **Converter:** `myks migrate` subcommand, `kcl import`-seeded, hand-finish guided by docs.
- **Smart mode:** deferred — KCL mode initially renders fully; decision 2's tree contract reserves
  its place.
