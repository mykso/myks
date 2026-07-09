# Configuration-layer requirements (redesign)

The requirement set the **config-language bake-off** is judged against. Scope is the *configuration
layer* — what replaces ytt data-values composition (ADR 0002). Engine/orchestration requirements are
settled by ADR 0001 and are **out of scope this session** (listed below for the boundary, not evaluated).

The bake-off implements the **full** language-deciding set in each candidate (KCL, CUE, Jsonnet)
**one at a time**, then judges on UX. Two comparison surfaces, **decoupled**:

- **Config level (this session):** each language's harness emits the *resolved configuration tree*;
  trees must match (same YAML structure). The tree then feeds the render engine — that seam is why
  the two surfaces decouple.
- **Render level (later, out of scope):** byte-identical manifests matter there, not here.

## Language-deciding requirements (under evaluation)

| # | Requirement | What it means | ytt today |
|---|-------------|---------------|-----------|
| L1 | **Configuration reuse / inheritance** | Values propagate down the tree root → cluster-group → tier → region → env → app; lower levels override/extend higher ones (last-wins deep-merge + list append). | ✅ (deep-merge only) |
| L2 | **Self-reference (intra-document)** | A value composed from a neighbour value in the same object (`arc_namespace` reused in two keys). | ❌ |
| L3 | **Cross-stage reference (intra-app)** | A value set in `app-data` decides another stage's input (`app-data` toggle → which `vendir/values` sources are included). | ❌ (double-toggle) |
| L4 | **Cross-application introspection (intra-env)** | An app derives config from *other apps in the same environment*: B targets A's namespace; alertmanager routes by every app's namespace+metadata; grafana dashboards render iff grafana-operator is in the env roster. Breadth = full read of siblings' resolved values. | ❌ (hand-hoisted shared keys) |
| L5 | **Shareable library** | Common logic factored into importable modules/functions, reused across prototypes. | ◐ (`.star`) |
| L6 | **First-class editor support** | LSP: completion, go-to-def, type errors in-editor. The #1 user pain. | ❌ (weak) |
| L7 | **Typing & validation** | Static schema, enums, min/max, conditional cross-field checks, nullable. | ◐ (ytt schema) |
| L8 | **Evaluation speed** | Full-tree evaluation must be fast enough that "eval everything, then fan out" is viable (see ADR 0003). Measured per language. | n/a |

## Engine requirements (settled — ADR 0001, NOT under evaluation)

Same regardless of which language wins. Listed for the boundary only.

- Idempotent rendering of manifests + supplementary assets
- Plugin system: render/source plugins enable/disable/reorder (the registry)
- Parallel execution of the fan-out (sync + render per app)
- Smart change detection (operates on the fan-out, diffing the resolved tree)

> **Speed** is the one cross-cutting item: parallel-render speed is engine (ADR 0001, out of scope),
> but **eval speed (L8) is a language property** and stays in the bake-off.

## Candidate languages

- **KCL, CUE, Jsonnet** — mandatory. All emit a resolved config tree → fit the config-level surface.
- **Pkl** — stretch 4th. Its eliminators (embed, helm-overlay) are now moot/decoupled; strong typing +
  LSP could score well on the rubric, so excluding it would bias the UX comparison.
- **cdk8s** — excluded. Generates k8s *manifests* from a general-purpose language, not a config tree;
  competes at the render level (decoupled), nothing to emit at the config level.

> Embed-ability is **not** a rubric dimension: ADR 0001's external-KRM-exec-plugin option makes
> in-process pure-Go non-decisive. This is what reopened the list past the `research-findings` ranking.

## Acceptance

Two tiers:

1. **Correctness gate (pass/fail).** Each language's harness emits the resolved configuration tree
   for the shared fixture (the existing `spike/tree/` fixture, *extended* with L3 cross-stage + L4
   cross-app cases). Gate = matches the **shared golden tree** after canonicalization (`yq sort_keys`)
   — same YAML structure. Golden = the ytt `--data-values-inspect` baseline where ytt can express the
   case; **hand-authored** for the new L3/L4 cases (ytt cannot produce them).
2. **UX rubric (scored — decides the winner among gate-passers).**

   | Dimension | Weight |
   |---|---|
   | User-surface readability | ×2 |
   | LSP / editor support (L6) | ×2 |
   | Eval speed, benchmarked (L8) | ×1.5 |
   | Error-message quality | ×1.5 |
   | Typing & validation power (L7) | ×1 |
   | Library / reuse ergonomics (L5) | ×1 |
   | Learning curve | ×1 |
   | Engine-harness complexity | ×0.5 |

### User surface

Built **pure-language** (every env level / prototype / app authored in the candidate language, *not*
plain YAML read by a harness) — this is the only way the ×2 rubric dimensions (readability, LSP) get
exercised at the surface the user edits. This **flips ADR 0002's "surface stays plain YAML"**, which
was chosen under the old byte-identical/minimal-migration framing. Leaf-override ergonomics are
*measured*, not assumed: where a language makes a one-line override ugly, that's a rubric signal and a
noted YAML-escape-hatch option.

**Inheritance is language-native (tried first).** Each level *imports/extends* its parent in-language
and apps import their prototype; the whole tree is one import graph evaluated in one pass (matches ADR
0003). No engine FS-walk + fold. Rationale: introspection (L4) falls out for free in one eval scope,
and go-to-definition across the inheritance chain becomes the honest LSP test (engine-fold hides it).
Import boilerplate at every level is **measured as a rubric signal**; if too heavy, the engine can
layer convention-discovery back on later. Reversible — ADR'd only if it survives the bake-off.

**Helm values are the one deliberate exception.** They're copied from upstream chart docs, so plain
YAML is near-unbeatable UX for the static case; the language only earns its keep when values need
logic. Each candidate builds **≥2 helm apps**:
- **static** — helm values supplied as plain YAML (tests the raw-YAML passthrough/embed ergonomics);
- **computed** — helm values derived in-language (cross-ref / conditional / introspection).

### Fixture scenario matrix

Reuse + extend `spike/tree/`'s apps; each maps to requirements:

| App | Exercises |
|---|---|
| `karma` (existing) | L1 inheritance, L7 typing/defaults, computed helm values |
| `central-forwarder` (existing) | L7 dense validation, **L3** cross-stage (vendir source from app flag), L4c grafana-operator presence |
| `grafana-operator` (existing) | presence drives L4c |
| `arc` (new) | **L2** self-reference (`arc_namespace` → 2 keys), multi-chart, **static helm = plain YAML** |
| supplementary app → main app ns (new) | **L4a** B targets A's resolved namespace |
| `alertmanager` (new) | **L4b** projection/comprehension over all apps' namespace + metadata mark |

> If the eval-speed benchmark (L8) shows full-tree eval is too slow across **all** candidates, the
> "eval everything" model (ADR 0003) is reconsidered.
>
> This bake-off **reopens ADR 0002** ("KCL primary"): that pick was settled on a byte-identical gate;
> the deciding instrument is now the UX rubric, so the winner is re-derived, not assumed.
