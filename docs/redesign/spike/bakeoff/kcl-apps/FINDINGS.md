# Track A findings — KCL (per-app layout)

Same language and semantics as `kcl/`, restructured so each leaf app is its own
subpackage. Gate: **PASS** (`kcl run kcl-apps/` matches `fixtures/golden.yaml`).

## Layout

```
kcl-apps/
  main.k                       3-line entrypoint (identical to kcl/)
  kcl.mod                      package bakeoff_kcl_apps
  lib/{schema.k,prototypes.k}  base + prototype schemas (unchanged from kcl/, minus arc's vendir default)
  envs/root/env.k onprem/env.k tools/env.k   upper levels (verbatim from kcl/ — infra apps stay inline)
  envs/stage/
    env.k                      leaf: env overrides + roster assembly + L4a/b/c introspection
    karma/app.k                p.Karma instance (L1 override, L6 config)
    central_forwarder/app.k    p.CentralForwarder instance (L7 validation, L3 vendir)
    arc/app.k  arc/vendir.k    app body + sync config split across two files in one package
    alertmanager/app.k         p.Alertmanager instance (routes layered at env scope)
    karma_dashboards/app.k     base s.App (namespace layered at env scope)
```

Only the **five leaf apps** moved to per-app files. Infra apps (cilium, namespaces,
goldpinger, cluster-autoscaler, grafana-operator, knot) stay inline in their owning
level's `env.k` — they are one-liners with no per-app config, so a dir each would be
pure ceremony. Per-app dirs earn their keep only where an app has real config to isolate.

## Proposed scorecard column — KCL (per-app layout)

| Dimension | w | Score | Justification |
|---|---|---|---|
| User-surface readability | x2 | 4 | Same clean inheritance as flat; per-app files aid *navigation* but the leaf `env.k` is now split from the app bodies, so reading one app end-to-end means two files (app.k + the env-scope patch). Net wash on readability, slight win on findability. |
| LSP / editor support (L6) | x2 | 4* | Identical to flat KCL — `kcl-language-server` ships completion/go-to-def/diagnostics. Cross-package go-to-def on `karma.app` works. Still scored from docs, not hands-on (x2 — confirm). |
| Eval speed (L8) | x1.5 | 5 | ~30 ms warm whole-tree (10 .k files). Faster wall-clock than flat's reported 66 ms; subpackage split adds no measurable cost. |
| Error-message quality | x1.5 | 4 | Same compiler as flat KCL; schema/check errors point at the right field. Unchanged. |
| Typing & validation (L7) | x1 | 5 | Schemas, unions, `check`, nullable all intact — apps instantiate typed prototypes in their own files, so typing survives the split. |
| Library / reuse (L5) | x1 | 4 | Prototypes in `lib/` reused across app files via `import`. Same as flat. |
| Learning curve | x1 | 3 | Flat's curve **plus** the package-path rules: dirs must be identifiers (`central_forwarder/`, not `central-forwarder/`), every app needs an explicit import + roster entry (no auto-discovery). More to know than flat. |
| Engine-harness complexity | x0.5 | 5 | `main.k` still 3 lines, no FS-walk. Wiring is in-language imports, not engine convention. |

`*` LSP not exercised hands-on; x2 weight — confirm before final.

Weighted total ~= **39.5 / 50** (vs flat KCL **40.5**). The split costs ~1 point on
learning curve; everything else holds.

## The A/B finding (the whole point)

**Per-app layout buys searchability, not readability, and charges import boilerplate for it.**

- **Findability win:** "edit central-forwarder" -> open `central_forwarder/app.k` directly,
  no scrolling a 73-line leaf file. `rg name.*karma kcl-apps/envs/stage/karma/` is scoped.
  This is the real myks UX and it is genuinely nicer for a human jumping to one app.
- **Readability wash / mild loss:** an app's *full* resolved shape is no longer in one place.
  Base instance lives in `<app>/app.k`; its env-dependent layer (karma's computed helm,
  alertmanager's routes, karma-dashboards' L4a namespace, CF's L4c flag) **must** stay in
  `env.k` because it needs cross-app / env scope. So reading karma end-to-end is now two
  files instead of one contiguous block. Flat `kcl/` had it all in `envs/stage/env.k` —
  longer file, but one read.
- **Import-boilerplate cost — quantified.** Adding a new leaf app costs **3 lines of wiring**
  on top of the app body:
  1. one `import envs.stage.<app> as <app>` line in `env.k`,
  2. one entry in the `_appList` append list,
  3. (if it needs cross-app/computed values) one entry in `_patched`.
  Plus a new directory and `app.k`. In flat `kcl/` adding an app was **1 line** (the
  instance) plus its append-list entry, same file — no import, no dir. So per-app layout
  is **~2–3 extra lines of pure wiring per app**, every app, forever. KCL has no
  subpackage auto-discovery, so this tax is unavoidable in-language (it's the cost of
  dropping the engine's FS-walk).
- **Total LOC:** 259 vs flat 217 (+42 lines, +19%) — entirely import lines, package
  docstrings, and the file boundaries themselves. Zero new logic.

**Verdict:** the per-app tree is the better *editing* UX (jump-to-app, smaller diffs,
matches real myks) but it is **not** a readability win in the language sense — env-scope
introspection can't move into the app files, so the leaf assembler stays just as dense and
every app now pays a fixed import tax. For KCL specifically the flat layout reads *better*
as a spike (one file, one eval scope visible at once); the per-app layout wins only once
the app count and per-app config size grow past what fits comfortably in one file. The
engine could later layer convention-discovery back to erase the import tax (requirements.md
already flags this) — without it, KCL's per-app story is "works, but you hand-wire every app".

## Eval time (L8)

`time kcl run kcl-apps/` warm: **~30 ms** (3 runs, all `real 0.03`). Flat `kcl/` reported
66 ms; both are noise at this fixture size — subpackaging is not a speed concern.

## Wins
- Direct jump-to-app navigation; scoped grep; small per-app diffs (real myks UX).
- arc's sync config (`vendir.k`) cleanly split from its body in the same package — shows
  the render/sync file split myks uses, with shared package scope so no re-import needed.
- Typing, validation, inheritance, eval speed all survive the split unchanged.

## Papercuts
- **Identifier dir names.** `central_forwarder/` not `central-forwarder/` (`expected
  identifier` otherwise); the real name lives in the body (`name = "central-forwarder"`).
  Same hyphen friction the flat findings noted, now also on directories.
- **No auto-discovery.** Every app = explicit `import` + roster entry. 2–3 lines of wiring
  per app, in `env.k`, forever.
- **Split-brain leaf.** Env-dependent app config (computed helm, L4 patches) can't live in
  the app file — it needs cross-app scope — so it stays in `env.k`. Per-app files hold only
  the env-independent base; the leaf assembler is still where the interesting wiring is.
- **Infra apps don't fit the model.** One-line infra apps would be all-ceremony as dirs, so
  they stay inline — the layout is only worth it for apps with real config.
