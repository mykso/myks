# Config-language bake-off — scorecard

Gate = resolved tree matches `fixtures/golden.yaml` after `yq -P 'sort_keys(..)'` + comment-strip
(`bash compare.sh '<emit cmd>'`). Winner decided by the UX rubric below, not the gate.

| Candidate | Gate | Built |
|---|---|---|
| **KCL (flat)** | ✅ PASS | ✅ full tree, single-file leaf assembler (`kcl/`) — comparison baseline |
| KCL (per-app layout) | — | ⬜ todo (`kcl-apps/`) — same language, app-per-file/dir; isolates file-tree UX |
| CUE | — | ⬜ todo |
| Jsonnet | — | ⬜ todo |
| Pkl | — | ⬜ todo (stretch) |

## Rubric (1–5, ×weight). Only gate-passers scored.

| Dimension | w | KCL | CUE | Jsonnet | Pkl |
|---|---|---|---|---|---|
| User-surface readability | ×2 | 4 | | | |
| LSP / editor support (L6) | ×2 | 4* | | | |
| Eval speed, benchmarked (L8) | ×1.5 | 5 | | | |
| Error-message quality | ×1.5 | 4 | | | |
| Typing & validation (L7) | ×1 | 5 | | | |
| Library / reuse (L5) | ×1 | 4 | | | |
| Learning curve | ×1 | 3 | | | |
| Engine-harness complexity | ×0.5 | 5 | | | |
| **Weighted total** | | **40.5 / 50** | | | |

`*` LSP scored from KCL *shipping* `kcl-language-server` (completion/go-to-def/diagnostics) — **not yet
exercised hands-on in an editor over this tree**. Confirm before the dimension is final; it is ×2.

## KCL findings (from building `kcl/`)

**Wins**
- **Language-native inheritance is clean.** Each level is `parent.env { overrides; apps += [...] }`:
  last-wins scalars, `+=` list append, nested-path override (`helm.common.global.tier = "stage"`) all
  native. **No hand-written deepMerge** — the old `spike/tree/` `merge.k` tax was an artifact of folding
  `yaml.decode`'d *files*; it vanishes when inheritance is in-language.
- **Import boilerplate is light:** one `import` per parent + per lib (~2–3 lines/level), absolute from
  module root (`import envs.tools`, `import lib.prototypes`). Dir tree == package tree, so it mirrors the
  myks env filesystem 1:1.
- **L2/L3/L4 fall out in one eval scope.** Self-ref = schema default over a sibling attr
  (`controllerNamespace: str = namespace`). L3 vendir = conditional dict-union in a schema default (one
  source of truth, double-toggle dead). L4a/b/c = comprehensions + dict lookups over the assembled roster.
- **Typing/validation (L7) is full:** schemas, `a | b` unions (one_of), `check` blocks, min, nullable,
  and conditional cross-field (`len(remoteWrite.url) > 0 if enabled`) — verified it *fires* on bad input.
- **Eval speed (L8): ~66 ms** whole-tree (217 LOC / 7 files), warm. Not a concern at this fixture size.
- **Engine-harness is trivial:** `main.k` is 3 lines; no FS-walk, no fold.

**Papercuts (rubric signal, not blockers)**
- **Re-validation on every override.** KCL re-evaluates derived defaults *and* `check` on each config
  override, so eager derivations can't tolerate the not-yet-set intermediate state — `Global.clusterFullName`
  + its region `check` needed guards (`... if region in _regionShort else ""`). This is the
  language-native form of old spike finding #2 ("check must not run on intermediate steps"). Alternative is
  to defer typed instantiation to the leaf, which weakens typed inheritance / LSP at upper levels.
- **Config-override won't parse on an index expression** inside a dict literal
  (`_byName["k"] { ... }`) — bind to a var first (`_alert { routes = ... }`). Minor.
- **Dotted shorthand is identifier-only;** hyphenated keys (`vm-agent`, `runner-set`) need explicit
  nesting (`"vm-agent" = {helmChart = {...}}`). Minor.
- **`sorted()` takes no key fn** (scalars only) — L4b name-sort is `sorted(keys)` then map-back. Workable.

## File layout & discovery — cross-language UX concern (applies to every candidate)

The current `kcl/` dumps **all** app configs in one leaf file (`envs/stage/env.k`). That is a
**spike-simplicity baseline**, not the intended UX. Real myks keeps each application (and its sync /
render config) in its **own directory/file** so you can jump straight to the thing you edit —
searchability is part of readability (a ×2 dimension). So file-tree shape is itself under evaluation.

Findings that shape it:

- **`_apps/` is an engine artifact, droppable here.** It exists today only so the engine's FS-walk can
  tell nested-environment dirs from application dirs. With **in-language wiring** (each level's file
  explicitly imports its apps) that disambiguation is in the import statements, not the directory name —
  so `_apps/` can go. A level dir can hold `env.<ext>` + child dirs that are sub-envs *or* apps; the
  wiring says which.
- **KCL has no subpackage auto-discovery.** Per-app files work (package = dir, many files share scope;
  subdir = importable subpackage), but the leaf assembler must explicitly `import` each app package and
  add it to the roster. Adding an app = dir + import line + list entry. This is the import-boilerplate
  cost the language pays for losing the engine's FS-discovery (requirements.md: "if too heavy, engine
  layers convention-discovery back later"). **Measure it per language** — Jsonnet `import`/`importstr`,
  CUE package auto-aggregation (CUE *does* union all files in a package — likely the lightest), Pkl
  `import*` glob may each weigh differently.
- **Identifier-safe names.** KCL package paths must be identifiers — a `central-forwarder/` dir is not
  importable (`expected identifier`); use `central_forwarder/` and keep `name = "central-forwarder"` in
  the body. Check each language for the same constraint.

Each candidate should pick the file tree that reads best for **its** import model — not blindly mirror
myks's `_apps/` layout. `kcl/` (flat) vs `kcl-apps/` (per-file) is the controlled A/B for KCL.

## What's reusable for CUE / Jsonnet / Pkl

- `fixtures/golden.yaml` — the shared oracle (language-agnostic).
- `compare.sh` — pass each language's emit command; same gate.
- `fixtures/scenarios.md` — the concrete app/requirement mapping to reproduce.
- Open per-language questions to answer by building: import-boilerplate weight, leaf-override ergonomics,
  L4b comprehension readability, whether whole-tree eval stays fast (L8).
