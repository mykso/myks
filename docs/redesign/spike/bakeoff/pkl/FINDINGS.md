# Pkl findings (Track D)

Gate: **PASS** — `nix shell nixpkgs#pkl nixpkgs#yq-go -c bash compare.sh 'pkl eval -f yaml pkl/main.pkl'`.
Pkl 0.31.1 (JVM, Java 21). Whole tree, 11 apps, all L1–L8 in one eval. 225 LOC / 5 files
(flat layout, app parity with `kcl/`).

## Proposed scorecard column

| Dimension | w | Pkl | Justification |
|---|---|---|---|
| User-surface readability | x2 | 4 | Typed classes + `amends` read clean; `helm { common { global { ... } } }` nesting is tidy. Minor noise: `new` keyword everywhere, `Dynamic` for any-typed passthrough. |
| LSP / editor support (L6) | x2 | 4* | Pkl ships a maintained LSP + IntelliJ/VSCode plugins (`pkl-lsp`). Not exercised hands-on here — scored from docs/availability, same caveat as KCL. |
| Eval speed, benchmarked (L8) | x1.5 | 2 | ~0.70s warm / ~1.05s cold, **100% JVM startup** (a `x = 1` file costs the same). Tree eval itself is sub-ms. ~10–15x slower wall-clock than KCL's 66 ms. Fine for one-shot CI, a tax for tight edit loops. |
| Error-message quality | x1.5 | 5 | Best of the field. Type errors show the offending value, the constraint expression, and carets under the failing sub-expression. |
| Typing & validation (L7) | x1 | 5 | Full: classes, `a \| b` string-literal unions (one_of), `Int(this >= 1)` inline constraints, `T?` nullable, conditional cross-field via hidden constrained property. Verified all fire on bad input. |
| Library / reuse (L5) | x1 | 4 | Classes + module `amends` + `import` give clean reuse. Rich stdlib (`List`, `Map`, `Mapping`, `sortBy` with key fn). |
| Learning curve | x1 | 3 | Three object kinds (typed class instance / `Dynamic` / `Listing`/`Mapping`) and the `const`/`local`/`hidden` modifier rules are non-obvious; several first-eval errors were modifier/scoping (see papercuts). |
| Engine-harness complexity | x0.5 | 5 | `pkl eval -f yaml main.pkl`, one command, no FS-walk, no fold, native YAML renderer. |

`*` LSP scored from availability, not hands-on — confirm before finalizing (x2 dimension).

## Eval time (L8) — the headline unknown, answered

```
min (x=1) startup floor : 0.68–0.72 s
main (full 11-app tree) : 0.70–0.72 s   warm
                          1.05–1.13 s   cold
```

**Verdict: startup-bound, not tree-bound.** A one-property file evaluates in the same wall-clock as
the whole resolved tree, so the JVM boot is the entire cost; the actual L1–L8 resolution is
negligible at this fixture size. There is no `pkl` daemon reuse across separate CLI invocations, so
every render pays the full JVM boot. Compared to KCL's ~66 ms this is ~10–15x slower in wall-clock.
For a once-per-render CI step it's a non-issue (~1 s); for an interactive edit/preview loop it's a
real papercut. A native-image (GraalVM) `pkl` build would likely erase most of it, but the nixpkgs
binary is JVM-backed.

## Typing / classes / amends inheritance — clean

Env-level inheritance reads as well as KCL. Each level amends the parent's `env` object; scalar
override is last-wins, and **amending a `Listing` appends** — no `+=`, no deepMerge helper:

```pkl
// onprem.pkl
env: Dynamic = (root.env) {
  kubernetesDistribution = "rke2"        // L1 override
  rancherEnabled = true
  apps {                                  // amend Listing = append
    new lib.App { name = "goldpinger"; proto = "goldpinger"; namespace = "monitoring"; monitored = true }
  }
}
```

Nested-path override is `helm { common { global { tier = "stage" } } }` — appends/narrows, never
clobbers siblings. Apps extend their prototype via `class Karma extends App`, instantiated with
`new lib.Karma { replicas = 2 }`. L2 self-ref is a plain default-to-sibling
(`controllerNamespace: String = namespace`). L3 vendir uses `when (centralForwarder.enabled) { ... }`
inside the object literal — one source of truth, no double-toggle. **No guards needed on derived
fields** (unlike KCL): Pkl is lazy, so `clusterFullName` simply isn't forced at intermediate levels.

L4 introspection is idiomatic functional code: `appList.any(...)` (L4c), `byName["karma"].namespace`
(L4a), and `filter().sortBy((a) -> a.name).map(...)` (L4b — **Pkl's `sortBy` takes a key function**,
so the L4b name-sort is one expression, unlike KCL's `sorted(keys)`-then-map-back).

## Discovery / file-layout — `import*` glob is the real win

`import*("apps/*.pkl")` auto-discovers every matching file and returns a `Mapping` keyed by path
(verified). **Cost to add a new app in a per-app layout = drop one file. Zero import lines, zero
roster edits.** This is strictly cheaper than KCL (dir + `import` line + list entry per app) and
matches CUE's package auto-aggregation.

This submission uses the **flat** layout (apps inline in `main.pkl`) for parity with the `kcl/`
baseline — but the glob is the lever a per-app Pkl layout would pull, and it removes the
import-boilerplate tax KCL pays. Note: the glob result is a `Mapping`, so you iterate `.keys`; and
file/dir names are free-form strings (no identifier constraint like KCL's `central_forwarder/`).

## Wins

- **Error messages** are the best in the bake-off — value + constraint + caret under the fault.
- **No deepMerge, no guards.** Listing-amend-appends and lazy eval kill the two biggest KCL papercuts
  (manual list merge tax was already gone in KCL; the derived-field guard tax is gone here too).
- **`sortBy` with a key fn** — L4b is one clean expression.
- **`import*` glob** — cheapest discovery story alongside CUE; per-app file adds nothing but the file.
- **Strong, expressive typing** — unions, inline `Int(this>=1)` constraints, nullable, conditional.

## Papercuts (rubric signal)

- **JVM startup dominates eval** (~0.7–1.1 s flat). The one quantified weakness. Edit-loop tax.
- **Three object flavors.** Typed class instance vs `Dynamic` (for L6 any-passthrough / the `env`
  blob) vs `Listing`/`Mapping`. Choosing the wrong one is a first-eval error.
- **Modifier rules bite early.** A class member referencing a module property needs that property
  `const`; properties inside a `Dynamic` literal **cannot** carry a type annotation (`apps =
  new Listing<App> {}`, not `apps: Listing<App> = ...`); name-shadowing inside an amend block (`routes
  = new { for (r in routes) ... }`) silently self-references and stack-overflows — rename the local.
- **Conditional-constraint message is weak.** The cross-field check fires correctly but reports
  `Type constraint ` + "`this`" + ` violated` (no custom message on type constraints), versus the rich
  messages on `Int(this>=1)` and unions.
- **Hidden-field idiom for cross-field validation.** Expressing "URL required when enabled" needs a
  `hidden urlRequired: Boolean(this) = ...` property so the check runs but isn't emitted — works, but
  less direct than KCL's `check:` block.
