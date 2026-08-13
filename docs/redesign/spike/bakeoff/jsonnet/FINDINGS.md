# Jsonnet — bake-off findings (Track C)

Gate: **PASS** — `bash compare.sh 'jsonnet jsonnet/main.jsonnet | yq -P'` matches `fixtures/golden.yaml`.
217 LOC across 6 files (`lib.libsonnet` 88, `envs/{root,onprem,tools,stage}.libsonnet` 21/12/13/80, `main.jsonnet` 3).

## Proposed scorecard column

| Dimension | w | Jsonnet | One-line justification |
|---|---|---|---|
| User-surface readability | ×2 | **4** | Clean object literals, `+`/`+:` inheritance reads well; `local` ceremony + `[a.name]:` comprehensions slightly noisier than KCL. |
| LSP / editor support (L6) | ×2 | **2** | `jsonnet-language-server` exists (hover/goto/diagnostics) but no type info to complete against — untyped objects mean completion is structural-guess at best. Scored from docs, not hands-on. |
| Eval speed, benchmarked (L8) | ×1.5 | **5** | ~15 ms wall warm incl. process spawn (`time jsonnet main.jsonnet` ≈ 10 ms). Trivially fast at this size. |
| Error-message quality | ×1.5 | **3** | Custom `assert` messages are good and fire correctly; but errors are *runtime*, point at the assert site not the bad caller, and only the first failure shows. No "field X has wrong type" before eval. |
| Typing & validation (L7) | ×1 | **2** | **The gap (headline below).** No schemas, no types, no field-presence checks. All validation is hand-written `assert`; structure is unenforced. |
| Library / reuse (L5) | ×1 | **4** | Prototypes as constructor functions compose cleanly; `import` graph is the reuse unit; rich `std` lib. No package registry story like KCL's, but fine here. |
| Learning curve | ×1 | **4** | Small language, JSON superset, one merge gotcha to learn. Easier than KCL/CUE; `self`/`$`/`+:` semantics are the only trap. |
| Engine-harness complexity | ×0.5 | **5** | `main.jsonnet` is 3 lines, one-pass eval, no FS-walk. Identical to KCL. |

## HEADLINE: the L7 typing/validation gap (honest characterization)

**What `assert` gets you:** value-level invariants with custom messages, evaluated at runtime.
I expressed all three L7 checks the KCL `check` blocks have:
- one_of: `assert std.member([...], mode)` -> fires `cf.mode must be one of ... got 'bogus'`.
- min: `assert shards >= 1` -> fires `cf.shards must be >= 1, got 0`.
- conditional cross-field: `assert !enabled || std.length(remoteWriteUrl) > 0` -> fires `set remoteWrite.url when enabling the forwarder`.

All three fire (verified). So *value* validation parity with KCL is achievable.

**What you cannot express, that KCL/CUE/Pkl give for free:**
1. **No type declarations.** `karma.replicas` is an int only because the default is `2`; pass `"2"` and nothing complains until something downstream coerces or breaks. KCL's `replicas: int` rejects it at the field. To get the same in Jsonnet you'd hand-write `assert std.isNumber(...)` per field — validation becomes O(fields) of boilerplate instead of free from the schema.
2. **No structural/shape checking.** A typo'd field (`replicaz: 2`) is silently accepted as an extra key — there is no closed-struct concept. KCL/Pkl reject unknown fields; CUE unifies and would catch a conflict. Jsonnet has nothing.
3. **No required-field enforcement.** A constructor arg can be defaulted-away or forgotten; only an explicit assert catches it. KCL `name: str` (no default) is a required field by construction.
4. **Errors are runtime + first-failure-only + located at the assert, not the offending call site.** The stack trace points at `lib.libsonnet:(41:5)` (the assert) plus the call line — usable, but it tells you *which rule* failed, not *which field of which app* in domain terms unless your message says so. KCL/CUE report all violations and tie them to the field path.

**Verdict:** Jsonnet can *reach* KCL's validation behavior for the specific invariants you bother to write, but only by hand, and it stops at value checks — there is no type system, no closed structs, no field-presence guarantees, and no pre-eval checking. For a config layer whose whole point is catching misconfig before render, that is a real, structural disadvantage versus KCL/CUE/Pkl. Rated **2/5**.

## List-merge helper (the one stdlib gap)

`+` deep-merges objects but **replaces** arrays, and `std.mergePatch` does the same. Env levels append to `apps`, so each appending level concatenates explicitly:

```jsonnet
apps: parent.apps + [ lib.app(...), lib.app(...) ],
```

That's the whole "helper" — there is no helper, just `parent.apps + [...]` inline. I added a named `appendApps` in lib but it's a one-liner (`base + extra`) and the env files use the inline form because it reads clearer. **Cost:** one `parent.apps +` per appending level (3 levels) vs KCL's native `apps += [...]`. Trivial, but it's a manual step you must remember — forget the `parent.apps +` and the child *silently drops* every inherited app (no error, since arrays just replace). That silent-drop footgun is the real tax, not the typing.

## Discovery / file-layout

Per-app authoring is constructor calls in `envs/stage.libsonnet`, not per-app files — at 11 apps, a file each is more wiring than payoff. Env *levels* are one file each (mirrors KCL `envs/<level>/`), wired by explicit `import`.

**Cost to add a new app:** 1 line (a `lib.app(...)` / prototype call) + add it to the `appList` concat (already one expression) = **~1-2 lines, one file**. No import needed unless it's a new prototype (then +1 `import` or +1 constructor in `lib`). Lighter than KCL-per-app (dir + import + roster entry), because Jsonnet has no package-identifier constraint and apps are values, not packages. Hyphenated keys are fine as quoted object keys (`'vm-agent':`, `'runner-set':`) — no identifier-safe-rename tax like KCL.

## Eval time (L8)

`time jsonnet main.jsonnet` warm ~= **10 ms** user; ~15 ms wall averaged over 20 runs including process spawn. Same ballpark as KCL (~66 ms reported — Jsonnet is actually faster here). Not a concern at this fixture size.

## Wins

- **Inheritance is native and clean:** `parent + { overrides; helm+: {...} }`. `+:` deep-merge handles the nested `helm.common.global` path without a hand-written merge.
- **`std.sort` has a key fn** (L4b): `std.sort(filtered, function(a) a.name)` — one call, no sort-keys-then-map-back dance KCL needs. Direct advantage over KCL here.
- **L2/L3/L4 fall out in one scope** exactly like KCL: self-ref via shared local, conditional dict-union for L3 vendir, comprehensions + `byName` lookup for L4a/b/c.
- **Hyphenated keys are free** (quoted object keys) — no identifier rename.
- **Tiny, fast, JSON-superset** — low harness complexity, low learning curve.

## Papercuts (evidence-based)

- **Silent array-replace on merge** (above) — the single sharpest footgun; forgetting `parent.apps +` drops apps with no error.
- **No typing** — validation is all hand-rolled asserts; structure is unchecked (headline).
- **Runtime, first-failure-only errors** located at the assert, not the bad value's field path.
- **`local` ceremony**: the leaf needs ~10 `local` bindings to stage intermediate values; reads slightly busier than KCL's schema-attribute style.
- **LSP can't do much without types** — completion is structural guessing.
