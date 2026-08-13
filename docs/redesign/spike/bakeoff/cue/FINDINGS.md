# CUE findings (from building `cue/`)

Gate: **PASS** — `cue export ./cue --out yaml` equals `fixtures/golden.yaml` after canonicalization.
Eval (L8): **~10 ms** warm (`time cue export ./cue --out yaml`, 4 files / ~210 LOC). Fastest of the field so far.

## Proposed scorecard column

| Dimension | w | CUE | One-line justification |
|---|---|---|---|
| User-surface readability | x2 | 4 | App/prototype structs read clean; the per-field `#last` override list is explicit but slightly verbose vs KCL `parent.env { x = ... }`. |
| LSP / editor support (L6) | x2 | 3 | `cue lsp`/`cuepls` exists (completion, diagnostics) but younger/less polished than KCL's; scored from docs, not exercised here. |
| Eval speed, benchmarked (L8) | x1.5 | 5 | ~10 ms whole-tree, warm. |
| Error-message quality | x1.5 | 4 | Conflict errors point at exact file:line of both sides; default-disjunction bound errors ("conflicting 2 and 0") read indirectly. |
| Typing & validation (L7) | x1 | 5 | Disjunction unions, `>=1` bounds, `!=""`, conditional (`if enabled {...}`) all fire on bad input — verified. Strongest constraint surface of the set. |
| Library / reuse (L5) | x1 | 4 | `import "list"` stdlib (Concat/Sort/Contains) covers L4; module/registry story exists. |
| Learning curve | x1 | 2 | Unification-not-mutation is a genuine mental-model shift; the no-override gotcha bites immediately (see below). |
| Engine-harness complexity | x0.5 | 5 | No entrypoint code at all — `cue export <dir>` aggregates the package. Lightest discovery story. |

## KEY finding — modeling override without override

CUE unification only **narrows**: `x: 1 & x: 2` is an error, and a struct comprehension that
re-emits a duplicate key conflicts (verified — `merged.x: conflicting values 2 and 1`). So
**last-wins inheritance is not struct unification** and you cannot "mutate a parent".

The clean idiom I landed on (`levels.cue`):

```cue
#last: {_v: [_, ...], out: _v[len(_v)-1]}

_levels: {
    kubernetesDistribution: (#last & {_v: ["gke", "rke2"]}).out   // root, onprem
    rancherEnabled:         (#last & {_v: [false, true]}).out     // root, onprem
    tier:                   (#last & {_v: ["stage"]}).out         // stage
    ...
}
```

Each overridable scalar is a **shallow->deep list of the values each level sets**; `#last` picks the
deepest. Override is explicit, one line per field, and reads as a vertical "override history" — you see
*who set what* at a glance, which the KCL mutate-the-parent form hides. Lists (the app roster) compose by
`list.Concat` then a remove-filter (`[for a in _allApps if a.proto != "bootstrap" {a}]`) — list "override"
is concat + filter, not merge.

**Verdict: clean, not ugly — but it inverts the model.** You author *data flow*, not *mutation*. The
upside is real and not just cosmetic: because nothing is re-evaluated on override (there is no override),
the KCL papercut of "derived defaults + `check` re-fire on every intermediate level and need guards"
(`clusterFullName` needing `if region in _regionShort`) simply **does not exist**. `#Global.clusterFullName`
reads the final unified `region` exactly once. The trade is conceptual load (low readability/learning-curve
score), not code volume.

Gotchas hit while building:
- `centralForwarder: #CF | *{}` left `.enabled` undefined for the L3 derivation — a `*{}` default isn't a
  `#CF`, so its fields aren't visible. Fix: `centralForwarder: #CF` (the def's own field defaults make it
  concrete). General lesson: don't hide a typed struct behind a bare-`{}` default if you need to read into it.
- `applications` is a map; "patched instance else roster entry" is `[if _patched[n] != _|_ {_patched[n]}, a][0]`
  — CUE has no ternary, so a single-element-comprehension-in-a-list + `[0]` is the idiom. Minor.

## Discovery / file-layout story — CUE's standout

**Package auto-aggregation eliminates import boilerplate entirely.** Every `.cue` file under `cue/` with
`package bakeoff` is unified automatically — no per-app `import`, no roster-registration `import` line,
no entrypoint that wires files together. Contrast KCL (per-app: dir + import line + roster entry) and the
SCORECARD File-layout concern: CUE pays the *least* of any candidate here.

**Cost to add a new app:** one struct in `apps.cue` (~3-6 lines) + one entry in the `_leafApps` list.
No import, no registration. The `_leafApps` list is the only seam (CUE won't auto-collect structs by
shape) — you could even drop apps in separate files; they still unify into the one package scope.
File-tree shape is therefore a free choice, not a wiring tax.

## L4b comprehension readability

```cue
_routes: [
    for n in list.Sort([for a in _roster if a.monitored && a.name != "alertmanager" {a.name}], list.Ascending)
    {receiver: n, namespace: _byName[n].namespace},
]
```

`list.Sort(..., list.Ascending)` takes a comparator — cleaner than KCL's `sorted()`-then-map-back
(KCL `sorted()` has no key fn). The nested comprehension reads top-to-bottom: filter+project to names, sort,
then build route structs. Good.

## Wins
- Zero engine/discovery boilerplate (package auto-union). Lightest file-layout story of the bake-off.
- No intermediate-state re-validation problem — derivations read final values once. Eliminates the KCL guard tax.
- Fastest eval (~10 ms).
- Strong, composable validation: unions, bounds, conditional constraints all fire (verified negatively).
- `list.Sort` with comparator beats KCL's keyless `sorted()` for L4b.

## Papercuts
- **No override = mental-model tax.** The single biggest learning-curve cost; the `#last` list is the price
  of admission. Anyone reaching for "just set `x: rke2` on top of root" hits a conflict error first.
- **Default-disjunction bound errors are indirect:** `shards: int & >=1 | *2` with `shards: 0` reports
  `conflicting values 2 and 0` (the default branch), not `out of bound >=1`. The `!=""` form reports the
  bound cleanly; numeric-with-default does not.
- **No ternary** — conditional selection is the `[...][0]` list idiom.
- **`_`-prefix for "computed, don't emit"** is a convention you must remember per field (`_levels`, `_roster`),
  vs KCL where you choose what the entry struct exposes.
