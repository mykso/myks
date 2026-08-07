# Spike — KCL vs CUE on data-values COMPOSITION

Settles the config-language pick (ADR 0002). The thing we are replacing is **ytt's data-values
composition**, NOT ytt's templates (the templates are fine and stay). myks collects every
`*-data.*.yaml` file on an Application's inheritance path (root → cluster-group → tier → leaf env
→ prototype), hands them to ytt, and ytt deep-merges them into ONE flat struct
(`ytt … --data-values-inspect`). That struct is computed in pass 1, so no field can derive from
another — the root pain (`CONTEXT.md` → "Data values").

So the operation under test is: **last-wins deep-merge of a 4-level, concrete-valued inheritance
tree + list-append + schema/typing + the derivation ytt cannot do.** We re-expressed the real
`karma` / `on-prem/tools/stage` chain in each language.

```
compose/
  ytt/   baseline — the 5 layer files + schema, merged with `ytt --data-values-inspect`
  kcl/   compose.k
  cue/   compose.cue
```

## Run it

```shell
cd compose/ytt && ytt -f 00-schema.ytt.yaml -f 10-root.ytt.yaml -f 20-on-prem.ytt.yaml \
  -f 30-tools.ytt.yaml -f 40-stage.ytt.yaml -f 50-proto-karma.ytt.yaml --data-values-inspect
nix shell nixpkgs#kcl -c kcl run compose/kcl/compose.k
nix shell nixpkgs#cue -c sh -c 'cd compose/cue && cue eval compose.cue -e environment'
```

All three emit the same merged struct. The difference: in ytt, `clusterFullName`
(`tools-stage-eu-dc1`) and `runsGrafanaOperator` are **hand-written** (a hardcoded string and a
hoisted shared key); in KCL and CUE both are **derived** — `clusterFullName` from
`{clusterGroup}-{tier}-{region_short[region]}`, `runsGrafanaOperator` from whether the
`grafana-operator` app is in the composed list. That derivation is the whole point: it is what
kills the hardcoded-drift and the shared-key/double-toggle workarounds.

## The decisive axis: last-wins override down a tree

myks env-data files set values **concretely** and a lower level **overrides** a higher one
(`kubernetesDistribution: gke` at root → `rke2` at on-prem). That is the core operation.

| | last-wins override | list append across layers | derivation | typing/validation |
|---|---|---|---|---|
| **KCL** | native — `gke`→`rke2` just works | `+=` | computed schema attrs | schema + `check` |
| **CUE** | **conflict** — `conflicting values "rke2" and "gke"` | not under `&` (needs `list.Concat`) | comprehension/interp | schema + `cue vet` |

Proof (`/tmp` scratch, reproduced in the spike notes):

```
# CUE: two layers, concrete override
_root: {kubernetesDistribution: "gke"} & {kubernetesDistribution: "rke2"}
  => out.kubernetesDistribution: conflicting values "rke2" and "gke"
# KCL: same scenario
{kubernetesDistribution = "gke"} | {kubernetesDistribution = "rke2"}  => "rke2"
```

**KCL** is the myks semantics out of the box. **CUE** is structurally mismatched: to layer at all
you must re-model every overridable field as a `*default | type` disjunction so a later concrete
can win, and any field two layers both set concretely is a hard error, not an override. For a
4-level tree with hundreds of keys overridden at arbitrary levels, that restructuring lands on the
**user surface** (every `env-data` file), and the conflict-on-double-concrete is a standing
footgun.

## KCL composition findings (the frictions, and why they are contained)

KCL matches the semantics but the merge idiom took iteration to get right — worth recording:

1. **`|` deep-merges plain dicts at any depth, but REPLACES nested *schema* instances.** Folding
   typed `Environment {…}` layers silently dropped sibling fields
   (`clusterFullName: Undefined-stage-Undefined`). Fix: fold an `{str:any}` dict accumulator, cast
   to the typed schema **once** at the end.
2. **`check:` fires on every intermediate fold step**, where the config is legitimately incomplete
   → false failures mid-fold. Fix: keep the fold lax; the single final cast runs `check`/derivation
   on the composed result.
3. **Untyped fold needs an explicit `{str:any}` annotation** or KCL's type inference rejects a
   nested dict against an inferred scalar-ish value type.

Crucially these frictions live in the **engine harness** (≈10 lines, written once). The user-facing
layer files stay trivial path patches: `_d = _d | {helm.common.global.tier = "stage", applications
+= [...]}`. CUE's friction is the opposite — it is spread across the user surface.

## Conclusion

**KCL primary.** It matches myks's last-wins-override-down-a-tree semantics natively, adds the
typing + derivation that kill the hardcoded-drift and shared-key workarounds, and its merge
frictions are contained in the engine. CUE remains capable for non-conflicting merge + validation,
but override-down-a-tree — the core myks operation — fights its unification model and would push a
re-architecture of the config onto every file. Fallback is revisited in ADR 0002 (the corrected
framing reopens CUE-vs-Jsonnet, since Jsonnet's object model is also natively last-wins).

## Edge-case results (the `tree/` spike)

`tree/` reimplements a realistic myks filesystem (nested env tree + `_apps` + `_env` +
prototypes) and a discovery/merge harness, then runs each ytt data-values behaviour through
KCL and diffs against a ytt `--data-values-inspect` baseline (canonicalized via
`yq sort_keys`). Two Applications (`karma`, `central-forwarder`) both **PASS byte-identical**.
See `tree/README.md` to reproduce.

| # | Edge case | ytt mechanism | KCL result | Notes / workaround |
|---|-----------|---------------|------------|--------------------|
| 1a | List append down the tree | array-append (data-values default) | **pass** | `+` inside the engine deep-merge |
| 1b | Remove inherited app by key | `#@overlay/match by="proto"` + `#@overlay/remove` | **pass** | Runs DURING composition → KCL owns it under A′. Plain filter `removeApps`; KCL self-references the list, so it is *cleaner* than the ytt overlay, not harder |
| 2 | Override precedence (proto vs `_apps` vs env level) | last-wins merge | **pass** | engine deep-merge, last layer wins |
| 3 | Schema layering across levels | `#@overlay/match-child-defaults missing_ok=True` | **pass** | open base + per-proto schema supplies typed defaults (e.g. `port: 8080`) |
| 4a | Enum validation | `#@schema/validation one_of=[…]` | **pass** | KCL union type `"a" \| "b"`; rejects bad value |
| 4b | Min constraint | `#@schema/validation min=1` | **pass** | `check: shards >= 1` |
| 4c | Conditional cross-field | `#@schema/validation (…), when=lambda` | **pass** | `check: len(url)>0 if enabled`; fires correctly |
| 4d | Nullable scalar | `#@schema/nullable` (default becomes null) | **pass** | `field?: any = None` → emits `null`, matching ytt |
| 5 | Double-toggle / shared key | hand-mirror `prototype.*` + `application.*` | **pass** | `runsGrafanaOperator` DERIVED from app list; vendir `vm-agent` source DERIVED from the app flag (`doubletoggle.k`) — one source of truth |
| 6 | any-typed passthrough | `#@schema/type any=True` | **pass** | `config?: any` round-trips the free-form blob |
| 7 | null-to-remove | `CreateNamespace: null` | **pass** | `yaml.decode` null → KCL `None` → emits `null`, byte-identical to ytt (ytt preserves explicit null; it does NOT drop the key) |
| 8 | Computed value at data level | Starlark local + hand-written `clusterFullName`/`Address` | **pass** | DERIVED in schema from `clusterGroup/tier/region/octet`; the whole drift-killer. KCL ignores the hand-written values and recomputes |
| 9 | Starlark helpers | `deep_get/deep_set`, `app_used/proto_used`, overlay-matchers | **pass (1–2)** / **n/a (3)** | `app_used`/`proto_used` → `any_true([a.proto==x …])` (already used for edge 5); `deep_get` → optional access. `overlay-matchers` belong to job 3 (ytt match-mutate), **out of A′ scope** |

**Two frictions surfaced, both contained, neither a blocker:**

1. **`|` conflicts on decoded-file override** → the engine needs a hand-written recursive
   deep-merge (`merge.k`, ~12 lines). This *revises spike finding #1*: `|` deep-merges dict
   **literals** with `=`, not `yaml.decode`'d data. **Lives in the engine, written once; the
   user surface stays plain YAML.**
2. **`check:` fires on schema-default instances** (e.g. an auto-instantiated `Global {}` with
   empty `region`) → keep nested schemas free of `= X {}` defaults; validate on the final
   composed cast (spike finding #2, reconfirmed).

**Fallback signal (settles the CUE-vs-Jsonnet open question):** the hand-merge is **not**
KCL-specific — Jsonnet's `mergePatch` also replaces arrays and `+` is shallow, so Jsonnet
needs the *same* ~6-line recursive merge; CUE cannot override at all. So the merge friction
does **not** differentiate the candidates. What differentiates them is what the edge cases
exercised: typing, the four validation flavours, and derivation-with-checking — all of which
**KCL passes and Jsonnet cannot do** (no schema). **Therefore: KCL stays primary; the fallback
is Jsonnet *only if types are negotiable* (it matches the merge but loses all
validation/derivation), and CUE *only if a hard pure-Go-in-process mandate makes its embed
story decisive* (accepting the override-restructuring tax).**
