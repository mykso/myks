# Handoff — fan out the config-language bake-off (4 parallel tracks)

**Goal:** implement the config layer in the remaining candidates, **in parallel**, each as its own
self-contained dir under `docs/redesign/spike/bakeoff/`. KCL (flat) is done and PASSes — it is the
reference. Each track reproduces the **same resolved tree** (`fixtures/golden.yaml`) at a **pure-language
user surface**, then records UX findings.

Repo: `/Users/glashevich/code/github.com/mykso/myks`, branch `redesign-brainstorm-docs`.

## Read first (do not duplicate)

- `README.md` — what the bake-off is, how to run the gate.
- `fixtures/golden.yaml` — **the oracle**. The exact resolved tree your output must equal.
- `fixtures/scenarios.md` — concrete app ↔ requirement matrix (the apps to author + what each exercises).
- `SCORECARD.md` — rubric + **KCL findings** + the **File-layout & discovery** concern (read this — it
  governs how you choose your file tree).
- `kcl/` — the working reference implementation (flat leaf assembler). Mirror its *semantics*, not its
  file shape.
- `../../requirements.md` — full spec (requirements L1–L8, rubric weights, helm dual-case, user-surface rules).
- `../../../adr/0003-evaluate-whole-tree-then-fan-out.md` — whole-tree eval model.
- `../../../adr/0002-...kcl.md` — **reopened**; winner is re-derived from the rubric, not assumed.

## The gate (identical for every track)

```shell
cd docs/redesign/spike/bakeoff
nix shell nixpkgs#<tool> nixpkgs#yq-go -c bash compare.sh '<your emit command>'
# must print: PASS — resolved tree matches golden
```

`compare.sh` canonicalizes both sides (`yq -P 'sort_keys(..)'` + comment-strip) and diffs. PASS = data
identical. Your emit command prints the whole resolved tree as YAML to stdout (root key `environment:`).

## Shared constraints (all tracks)

1. **Pure-language surface.** Every env level + prototype + app authored in the candidate language — not
   plain YAML read by a harness. (Helm values are the one exception: see requirements §Helm dual-case —
   `arc` = static plain-YAML values, `karma` = computed-in-language values.)
2. **Language-native inheritance, tried first.** Levels compose root→onprem→tools→stage in-language;
   apps extend their prototype; whole tree = one import graph, one-pass eval. No engine FS-walk.
3. **Whole-tree output.** Emit one resolved tree for env `tools-stage-eu-dc1` with all 11 apps as a map
   under `environment.applications` (see golden). Cross-app introspection (L4a/b/c) resolves in one scope.
4. **Consider the per-app file layout, don't be bound by it.** Real myks puts each app in its own
   dir/file (searchability = readability, a ×2 dimension). Pick the tree that reads best for *your*
   language's import model. `_apps/` is an engine artifact — drop it; wire apps explicitly. Record the
   import-boilerplate / discovery cost (SCORECARD §File-layout).
5. **Record results:** add your column to the `SCORECARD.md` rubric table + a findings subsection
   (wins + papercuts, evidence-based). Note eval time (L8), and flag if LSP is scored from docs vs
   hands-on. Do **not** edit other tracks' dirs or the shared fixtures.
6. **Tools via `nix shell nixpkgs#<pkg>`** (no brew). `fd`/`rg` over `find`/`grep`.

## The four tracks (independent — run concurrently)

### Track A — `kcl-apps/` (new KCL, per-app layout)
Same language as `kcl/`, **restructured** to app-per-file/dir to isolate the file-tree UX variable.
Keep `kcl/` untouched (the A/B baseline). Suggested shape (adjust for best UX):
```
kcl-apps/
  kcl.mod
  lib/                 base + prototype schemas (one file per prototype)
  envs/root/env.k  envs/onprem/env.k  envs/tools/env.k
  envs/stage/env.k     env-scope + roster assembly (imports each app, does L4 introspection)
  envs/stage/karma/app.k  envs/stage/central_forwarder/app.k  envs/stage/arc/{app.k,vendir.k}  ...
```
Gotchas already known: KCL package paths must be identifiers (`central_forwarder/`, not
`central-forwarder/`); no subpackage auto-discovery (explicit import + roster entry per app). Compare
its readability/searchability against flat `kcl/` in the scorecard — that delta is the whole point.
Emit: `kcl run kcl-apps/`.

### Track B — `cue/`
CUE. Note: CUE **unifies all files in a package** automatically — likely the lightest "discovery" story
(no explicit per-app import; just drop files in the package). Lean into that. Watch: CUE cannot override
a concrete value (unification only narrows) — model inheritance via disjunctions / `*default` / explicit
override fields, not last-wins mutation. The old `../tree/` README notes CUE "cannot override at all" —
solving that cleanly (or proving it ugly) is the key CUE finding. Emit: `cue export ... --out yaml`.

### Track C — `jsonnet/`
Jsonnet (use `nixpkgs#go-jsonnet`). Inheritance = object `+` / `+:` deep-merge; lists need a manual
merge helper (`std.mergePatch` replaces arrays). L4b = `std.filter` + `std.sort` (has a key fn, unlike
KCL). No static typing (L7) — model validation via `assert`. That L7 gap vs KCL/CUE/Pkl is the headline
finding to characterize honestly. Emit: `jsonnet main.jsonnet | <to-yaml>` (or `-y`/yq).

### Track D — `pkl/` (stretch)
Pkl. Strong typing + classes + `amends` for inheritance; `import*` glob may give cheap discovery. Likely
strong on typing/LSP, unknown on eval speed (L8) — benchmark it. Emit: `pkl eval -f yaml main.pkl`.

## What to hand back

Per track: a PASSing dir + a filled SCORECARD column + findings. Then a synthesis pass (separate) ranks
the gate-passers by weighted rubric and writes the recommendation — **that** reopens/closes ADR 0002.

## Open questions (answer by building)

Import-boilerplate weight per language; leaf-override ergonomics; L4b comprehension readability; whether
whole-tree eval stays fast across all (if not across **all** → reconsider ADR 0003); does the per-app
file tree actually improve readability or just add wiring (the `kcl/` vs `kcl-apps/` A/B answers this).
