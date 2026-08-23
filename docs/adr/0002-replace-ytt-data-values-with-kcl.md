# Replace the ytt data-values engine with KCL (values + helm-templating only)

We will replace **ytt's data-values composition** with **KCL** as the configuration language for
the redesigned renderer, scoped to **value computation + helm-value templating**. ytt is
**retained** for match-and-mutate overlays. **CUE and Jsonnet are both kept on the table as the
fallback**; which one is not yet settled (see Status).

Status: **accepted (re-confirmed by bake-off).** The original KCL pick was settled on a
*byte-identical* gate; it was reopened and re-derived on a **UX rubric** over a full per-language
bake-off (KCL flat, KCL per-app, CUE, Jsonnet, Pkl — all five implemented the config layer and passed
the correctness gate; see `docs/redesign/requirements.md` and
`docs/redesign/bakeoff-results.md`). **KCL won: 44.0 vs CUE 41.0 = Pkl 41.0 > Jsonnet 36.5**
(max 52.5). The ×2 LSP dimension was subsequently verified hands-on in an editor (2026-08-23) — all
five language servers work as documented; scores stand. The fallback ordering below is **updated by
the bake-off**: CUE and Pkl are the near-tie runners-up; Jsonnet is no longer recommended (structural
typing gap, silent array-replace footgun). The rest of this ADR records the original reasoning and
spike evidence; sections contradicted by later findings carry inline notes.

Original status: **accepted.** The primary (KCL) is decided and the long-tail edge cases are closed — a
realistic-filesystem spike ran all flagged ytt data-values behaviours through KCL byte-identical
against a ytt baseline, with **no blocker** (two contained engine-level frictions; see below). The
original fallback ordering (Jsonnet if types are negotiable, CUE only under a hard
pure-Go-in-process mandate) is **superseded by the bake-off ranking above**. The then-open
host-language / fork-vs-rewrite decision was later settled by ADR 0005 (Go, evolve in place).
Spike code was removed from the repo after the decisions closed; the durable findings live in
`docs/redesign/bakeoff-results.md`.

## Scope — how much of ytt's job the language takes (decision A′)

ytt does three jobs: (1) compose the flat data-values struct from the inheritance tree, (2) template
helm values / generate resources from it, (3) match-mutate overlay the rendered helm output
(`global-ytt`, prototype `ytt/` overlays). We take option **A′**: the new language owns **jobs 1
and 2** — all value computation, where the root pain lives — and ytt **keeps job 3** (~180
`#@overlay/match` files in the reference production repo), its genuine strength and where every
candidate language is weakest. The
pipeline redesign (ADR 0001) already turns "ytt-overlay" into a registered mutative plugin, so this
is a clean `compute │ patch` seam. Rejected: **A** (values only — leaves helm templating stuck in
ytt) and **B** (language owns overlays too — biggest migration, highest risk to the byte-identical
gate, on the languages' weakest axis).

## Why KCL

The thing being replaced is **ytt's data-values composition**, not its templates (the templates
are good). myks collects every `*-data.*.yaml` on an Application's inheritance path (root →
cluster-group → tier → leaf → prototype) and ytt deep-merges them into one flat struct in pass 1 —
so no field can derive from another. The operation to reproduce is therefore: **last-wins
deep-merge of a 4-level, concrete-valued inheritance tree + list-append + schema/typing + the
derivation ytt cannot do.**

We settled it with a spike that re-expressed a real 4-level production inheritance chain in ytt
(baseline), KCL and CUE. All three emit the same merged struct;
in ytt `clusterFullName` and `runsGrafanaOperator` are hand-written, in KCL and CUE they are
**derived** (killing the hardcoded-drift and the shared-key/double-toggle workarounds). The
decisive axis is the core operation — last-wins override down the tree:

- **KCL matches myks semantics natively.** A lower layer overriding a higher concrete value
  (`kubernetesDistribution: gke` → `rke2`) just works; lists append with `+=`; typing + `check`
  validation + derived values all apply on the composed result.
- **CUE is structurally mismatched.** Two layers setting the same scalar concretely is a
  *conflict* (`conflicting values "rke2" and "gke"`), not an override. Making CUE layer at all
  forces re-modeling every overridable field as a `*default | type` disjunction — a change spread
  across the **user surface** (every `env-data` file) — and leaves a footgun: any field two layers
  both set concretely becomes a hard error.

KCL's merge does have frictions, confirmed and contained by the edge-case spike: (1) **`|`
conflicts on scalar override of `yaml.decode`'d data** — deep-merge-under-`|` holds only for dict
*literals* with `=`, so folding real env-data **files** needs a hand-written recursive deep-merge
(`merge.k`, ~12 lines: deep-dict + array-append + last-wins); (2) `check:` fires on schema-default
instances, so nested schemas avoid `= X {}` defaults and validation runs on the final composed
cast. Both live in the **engine harness** (written once). KCL also brings the #1-pain fixes:
first-class LSP (`kcl-language-server` is packaged), Python-shaped syntax, static schemas.

> **Note (post-bake-off):** the two frictions above and the "user-facing layer files stay plain
> YAML" framing belong to the file-folding model. ADR 0004 switches the user surface to
> pure-language with in-language inheritance, under which the hand-written deep-merge **vanishes**
> (it was an artifact of folding `yaml.decode`'d files) and the friction reduces to
> derived-default/`check` guards on intermediate override states (see SCORECARD, KCL papercuts).

## Fallback — CUE vs Jsonnet (superseded by the bake-off)

> **Superseded:** the bake-off ranked the fallbacks **CUE (41.0) = Pkl (41.0) > Jsonnet (36.5)** and
> recommends against Jsonnet for this config layer (no types/closed structs/required fields; silent
> array-replace on merge). CUE's strength there was package auto-aggregation (lightest file-layout
> story) and no derived-field guard tax; Pkl's was best-in-field error messages, with JVM startup
> latency its only real weakness. See `docs/redesign/bakeoff-results.md`. The analysis below
> is the pre-bake-off reasoning, kept for the record:

- **CUE** — best types/correctness + pure-Go in-process embed. Best fallback **if** the trigger is
  a hard "embed pure-Go in-process" mandate from session 2 (accepting the override-restructuring
  tax above).
- **Jsonnet** — pure-Go (`go-jsonnet`); Tanka proves the env-inheritance pattern. The edge-case
  spike corrected one assumption: Jsonnet is **not** more native on the merge — `std.mergePatch`
  replaces arrays and `+` is shallow, so Jsonnet needs the *same* ~6-line recursive deep-merge KCL
  does. Its only real merge advantage over KCL is that it has no `|`-on-decoded-data conflict to
  work around. Cost: dynamically typed, **no schema/validation/derivation-with-checking** — exactly
  the capabilities the edge cases exercised and KCL passed. Best fallback **if** the trigger is KCL
  bus-factor and types are negotiable.

**Settled by the edge-case spike.** The hand-written deep-merge is **not** a KCL-specific tax (both
non-CUE candidates need it), so it does not differentiate. What differentiates is typing + the four
validation flavours + derivation — all of which KCL passed and Jsonnet cannot do. The ordering this
produced (KCL > Jsonnet > CUE) was later revised by the bake-off (see Status).

## The FFI caveat and the host-language implication (resolved)

KCL's one real knock is that its Go SDK is **not pure-Go** — it loads a prebuilt native library.
This was flagged as a potential host-language trigger; it is **resolved by ADR 0005/0006**: the
SDK loads the library via purego (no CGO), covers all release platforms, and the engine embeds it
in-process. The KRM ResourceList exec-plugin escape hatch (ADR 0001) remains available but is not
needed for the config layer. No pure-Go-in-process mandate materialized, so the CUE fallback
trigger never fired.

## Migration feasibility

- **Coexistence:** A′ lets KCL run alongside ytt during transition — KCL emits the composed
  data-values struct that ytt then consumes for templating/overlays — so migration is **app-by-app
  behind the byte-identical-output gate** (ADR 0001), not a big bang.
- **Importer:** `kcl import` ingests plain YAML/JSON and k8s CRDs, so the ~600 data-values files of
  the reference production repo (and CRD-derived schemas) can be machine-seeded; the typed schemas
  + derivation are authored on top.
- **Out of scope:** the `#@overlay/match` files stay in ytt under A′ and are not ported.

## Consequences

- Renderer config becomes two languages along a clean seam: **KCL** for compute, **ytt** for
  match-mutate overlays. Accept the two-language cost as the price of each tool on its strength.
- The host must run KCL in-process **or** as a KRM exec plugin — resolved by ADR 0005/0006:
  in-process via kcl-go.
- **Boundary clarified:** list remove-by-key (`#@overlay/remove` in `env-data.apps`) runs *during*
  data-values composition, so it falls inside KCL's job under A′ — it is **not** part of the job-3
  match-mutate overlays that stay in ytt. KCL expresses it as a list filter (it can self-reference
  the accumulated list, which ytt data values cannot), so this lands on the easier side of the seam.
- The flagged edge cases are **closed** by the realistic-filesystem spike (deep list merge + remove
  by key, schema inheritance across the tree, prototype→application precedence, all validation
  flavours, null-preservation, derivation): two Applications passed byte-identical against the ytt
  baseline, with the two engine-level frictions documented above. That harness is the template for
  the per-app byte-identical migration gate.
