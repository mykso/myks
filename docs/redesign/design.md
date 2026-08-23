# Config-layer redesign — implementation design

The design for replacing ytt data-values composition with KCL. Decisions behind it: ADR 0002
(KCL), ADR 0003 (whole-tree eval), ADR 0004 (pure-language surface), ADR 0005 (Go host, evolve in
place), ADR 0006 (engine integration). Evidence: [bakeoff-results.md](bakeoff-results.md).

## Big picture

Two phases (ADR 0003):

```
phase 1 — evaluate:  kcl.Run(<repo root>)  →  frozen resolved tree (JSON)
phase 2 — fan out:   per-app parallel sync + render, read-only over the frozen tree
```

The frozen tree is the **sole discovery mechanism**: environment list, app list, and every app's
resolved configuration are read from the evaluation output. The engine performs no filesystem walk
to discover environments or applications; the user's in-language imports (ADR 0004) are the wiring.

## User surface

### Tree layout

```
envs/
  env.k                      root level: base schema instance, global defaults, root apps
  <group>/
    env.k                    cluster-group level: imports parent, overrides, apps += [...]
    <tier>/
      env.k                  tier level: same pattern
      <region>/
        env.k                leaf: identity values (region, octet, …) + finalize
        <big_app>/app.k      optional per-app opt-in for apps with substantial config
lib/                         user-local shared logic (importable)
kcl.mod                      KCL module manifest; pins the myks schema package
```

- **Flat is the default; per-app directories are a per-app opt-in** (bake-off addendum). Both
  styles coexist in one tree.
- Directory names must be KCL identifiers (`central_forwarder/`, not `central-forwarder/`);
  display names live in the config body (`name = "central-forwarder"`).
- No `_apps/` convention: imports disambiguate sub-environments from applications (ADR 0004).
- Variable-depth trees work: intermediate directories need no `.k` files; a child package can
  import its ancestor.

### Authoring pattern

Each level imports its parent and patches it; leaves run a shared finalizer:

```kcl
# envs/shop/prod/east/env.k — a plain leaf, everything else derives
import envs.shop.prod as parent
import myks.finalize as f

env = f.finalize(parent.env {
    global.region = "east"
    global.octet = 34
})
```

Per-cluster override of one inherited app (flat style):

```kcl
_lvl = parent.env {
    global.region = "east"
    global.octet = 40
}
_promNs = {namespace = "monitoring"}
env = f.finalize(_lvl {
    apps = [(a | _promNs if a.name == "prometheus" else a) for a in _lvl.apps]
})
```

KCL idioms and papercuts to respect (details in
[bakeoff-results.md](bakeoff-results.md#key-kcl-findings-carried-into-designmd)):

- Derived defaults and `check` blocks re-evaluate on every override — guard derivations against
  not-yet-set intermediate state, or validate only in the finalizer.
- Patch list elements with dict-union (`a | _patch`), not config-override on a typed variable.
- Keep nested schemas free of eagerly-instantiated defaults so `check` doesn't fire on
  intermediates.

### Helm values

Static helm values copied from upstream chart docs stay **plain YAML** files, passed through
(ADR 0004's one exception). Values that need logic (cross-reference, conditionals, introspection)
are computed in KCL and merged by the engine over the static YAML.

## myks schema package

myks ships its KCL schemas (`Environment`, `App`, prototype bases, the `finalize` helper) as a
**KCL package on an OCI registry** (ADR 0006). User repos depend on it via `kcl.mod`
(`kcl mod add …`), pinned by version; the LSP resolves it like any dependency. The evaluated tree
carries the schema version; the engine asserts compatibility at eval time and fails fast on
mismatch.

## Frozen tree contract

The root evaluation must emit one document shaped as:

```yaml
myksSchemaVersion: <semver>       # asserted by the engine
environments:
  <env id>:                       # e.g. "shop/prod/east"
    id: <cluster full name>
    argocd: {...}                 # env-level ArgoCD settings
    applications:
      <app name>:
        proto: <prototype name>   # selects sync/render sources
        sync: {...}               # vendir-equivalent source config
        render: {...}             # per-step config (helm values, ytt inputs, plugin args)
        <resolved values...>
```

Requirements on the shape (they exist so later phases bolt on without breaking it):

- **Per-app resolved config is a self-contained, diffable unit** — deferred smart-mode will diff
  these units between runs; nothing outside an app's entry may be needed to render it.
- Everything the fan-out consumes (sync sources, helm values, ytt data-values, ArgoCD app fields)
  is present in resolved form; no back-references into KCL land after phase 1.

## Engine integration (ADR 0006)

- **Evaluation**: in-process via `kcl-lang.io/kcl-go` (native lib loaded via purego, no CGO;
  adds ~16 MB to the binary; kcl-go version is in lockstep with the KCL language version).
  Document `KCL_LIB_HOME` for read-only-rootfs environments — the SDK extracts its native lib to
  a writable cache dir on first run.
- **Mode detection is per-repo**: presence of the KCL tree marker (`kcl.mod` at the config root)
  selects the new path; otherwise the legacy ytt data-values path runs. No per-environment mixed
  mode.
- **Pipeline in KCL mode**: frozen tree → per-app fan-out → vendir sync (sources from tree) →
  helm template (values file generated from tree) → ytt overlay step → plugins → slice → ArgoCD
  app generation (fields from tree). Existing embedded tools (ytt, vendir, kbld) are unchanged.
- **ytt bridge**: for each app the engine writes the resolved config as a generated data-values
  YAML file and passes it to the ytt step (`--data-value-file`). Existing ytt templates and
  overlays keep reading `#@ data.values.…` unchanged — data-values *authoring* dies, data-values
  as ytt's input API stays.

## Migration

- **Converter**: `myks migrate` subcommand. Machine-seeds the KCL tree from an existing repo:
  `kcl import` for the plain-YAML data-values files, re-parenting onto the level/import structure,
  roster generation from the existing `_apps` layout. Hand-finishing (typed schemas, derivations)
  is guided by migration docs.
- **Gate**: byte-identical rendered manifests between legacy and KCL mode, waived per app only
  where derivations intentionally change values. The gate harness renders a repo in both modes and
  diffs `rendered/`; it doubles as the converter's test suite.
- **Legacy**: previous major frozen on a branch, bugfixes + trivial features backported on demand
  for 12 months from the first stable new-major release (ADR 0005).

## Deferred (designed around, not built now)

- **Smart mode**: new path ships with full render. Tree-diff smart mode bolts on later; the frozen
  tree contract above is what makes that possible without a schema break.
- **vendir/kbld replacement**: both stay embedded as-is; not required by the config layer.
