# Config-layer redesign — roadmap

Implementation sequence for [design.md](design.md). Each step is a reviewable PR chain; a step's
"done" makes the next step plannable from the docs alone. Solo + agent-assisted effort model:
incremental PRs, no big bang.

## 1. Walking skeleton

Minimal end-to-end new path proving the engine seam:

- `kcl-lang.io/kcl-go` dependency; per-repo mode detection (`kcl.mod` marker).
- `kcl.Run(root)` → frozen tree → fan-out reads envs/apps from it → existing sync/render per app
  with the ytt data-values bridge.
- One toy KCL-mode repo under `examples/`.

Done when: the toy repo renders through the new path; legacy repos are untouched.

## 2. Schema package

- Formalize the myks KCL schemas (`Environment`, `App`, prototype base, `finalize`) from the
  design doc.
- Publish as an OCI KCL package; wire the version assert into eval.
- The skeleton example consumes the published package via `kcl.mod`.

## 3. Full pipeline on real fixtures

- Port an `examples/` integration-test fixture to KCL mode.
- Wire ArgoCD app generation from the frozen tree; all render steps (helm, ytt overlays, plugins,
  slice) fed from tree values.
- Build the **byte-identical gate harness**: render a repo in both modes, diff `rendered/`. This
  harness is the migration gate and the converter's test suite.

Done when: the ported fixture passes the gate byte-identical.

## 4. `myks migrate`

- Converter subcommand: `kcl import`-seeded YAML→KCL conversion, re-parenting onto the
  level/import structure, roster generation from `_apps/`.
- Migration guide covering the hand-finish (typed schemas, derivations, per-app opt-in layout).
- Validated by the gate harness over the `examples/` fixtures.

## 5. Real-world migration

- Run converter + gate over a large production repo (out-of-repo activity; feeds converter fixes
  and guide improvements back here).

## 6. Release

- Cut the legacy branch; document the legacy policy (bugfixes + trivial features on demand,
  12 months from first stable new-major release — ADR 0005).
- New major release via release-please; migration guide + deprecation announcement.

## Deferred until after the above

- **Smart mode on the frozen tree**: diff per-app resolved config units between runs; add
  non-config triggers (prototype file changes, chart bumps). The tree contract in design.md
  already reserves the diffable per-app unit.
- **vendir/kbld replacement**: reimplement the needed subset in-repo when their limitations
  actually bite; design the sync-cache seam at that point.
