# Replace the ytt data-values engine with KCL (values + helm-templating only)

We will replace **ytt's data-values composition** with **KCL** as the configuration language for
the redesigned renderer, scoped to **value computation + helm-value templating**. ytt is
**retained** for match-and-mutate overlays. **CUE and Jsonnet are both kept on the table as the
fallback**; which one is not yet settled (see Status).

Status: **accepted.** The primary (KCL) is decided and the long-tail edge cases are closed — a
realistic-filesystem spike (`docs/redesign/spike/tree/`) ran all flagged ytt data-values
behaviours through KCL byte-identical against a ytt baseline, with **no blocker** (two contained
engine-level frictions; see below). The fallback ordering is now settled: **Jsonnet if types are
negotiable, CUE only under a hard pure-Go-in-process mandate.** Remaining open item is the
host-language / fork-vs-rewrite decision (session 2), not this language choice.

## Scope — how much of ytt's job the language takes (decision A′)

ytt does three jobs: (1) compose the flat data-values struct from the inheritance tree, (2) template
helm values / generate resources from it, (3) match-mutate overlay the rendered helm output
(`global-ytt`, prototype `ytt/` overlays). We take option **A′**: the new language owns **jobs 1
and 2** — all value computation, where the root pain lives — and ytt **keeps job 3** (~177
`#@overlay/match` files), its genuine strength and where every candidate language is weakest. The
pipeline redesign (ADR 0001) already turns "ytt-overlay" into a registered mutative plugin, so this
is a clean `compute │ patch` seam. Rejected: **A** (values only — leaves helm templating stuck in
ytt) and **B** (language owns overlays too — biggest migration, highest risk to the byte-identical
gate, on the languages' weakest axis).

## Why KCL

The thing being replaced is **ytt's data-values composition**, not its templates (the templates
are good). myks collects every `*-data.*.yaml` on an Application's inheritance path (root →
cluster-group → tier → leaf → prototype) and ytt deep-merges them into one flat struct in pass 1 —
so no field can derive from another (`CONTEXT.md` → "Data values"). The operation to reproduce is
therefore: **last-wins deep-merge of a 4-level, concrete-valued inheritance tree + list-append +
schema/typing + the derivation ytt cannot do.**

We settled it with a spike (`docs/redesign/spike/`) that re-expressed the real `karma` /
`on-prem/tools/stage` chain in ytt (baseline), KCL and CUE. All three emit the same merged struct;
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
cast. Both live in the **engine harness** (written once); the user-facing layer files stay plain
YAML. KCL also brings the #1-pain fixes: first-class LSP (`kcl-language-server` is packaged),
Python-shaped syntax, static schemas.

## Fallback — CUE vs Jsonnet (open)

Both are pure-Go-embeddable and stay candidates; the corrected framing changes the trade-off:

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
validation flavours + derivation — all of which KCL passed and Jsonnet cannot do. Ordering:
**KCL > Jsonnet (if types negotiable) > CUE (only under a hard pure-Go-in-process mandate, accepting
the override-restructuring tax).**

## The cgo caveat and the host-language implication (for session 2)

KCL's one real knock is that its Go SDK is **cgo/FFI, not pure-Go** — relevant only because the
host language and fork-vs-rewrite are still open (session 2). It is **neutralized**: ADR 0001
adopts **KRM ResourceList** as the external exec-plugin wire contract, and KCL ships an official
KRM-function plugin, so KCL can run as an external exec plugin and the host stays pure-Go.
**Flagged for session 2:** a hard in-process-pure-Go mandate is the trigger to fall back (to CUE,
or to Jsonnet if types are negotiable).

## Migration feasibility

- **Coexistence:** A′ lets KCL run alongside ytt during transition — KCL emits the composed
  data-values struct that ytt then consumes for templating/overlays — so migration is **app-by-app
  behind the byte-identical-output gate** (ADR 0001), not a big bang.
- **Importer:** `kcl import` ingests plain YAML/JSON and k8s CRDs, so the ~599 data-values files and
  CRD-derived schemas can be machine-seeded; the typed schemas + derivation are authored on top.
- **Out of scope:** the ~177 `#@overlay/match` files stay in ytt under A′ and are not ported.

## Consequences

- Renderer config becomes two languages along a clean seam: **KCL** for compute, **ytt** for
  match-mutate overlays. Accept the two-language cost as the price of each tool on its strength.
- Session 2 inherits one explicit dependency: pick a host that can run KCL in-process (cgo) **or**
  as a KRM exec plugin; only a hard pure-Go-in-process mandate flips the choice.
- **Boundary clarified:** list remove-by-key (`#@overlay/remove` in `env-data.apps`) runs *during*
  data-values composition, so it falls inside KCL's job under A′ — it is **not** part of the job-3
  match-mutate overlays that stay in ytt. KCL expresses it as a list filter (it can self-reference
  the accumulated list, which ytt data values cannot), so this lands on the easier side of the seam.
- The flagged edge cases are **closed** by `docs/redesign/spike/tree/` (deep list merge + remove by
  key, schema inheritance across the tree, prototype→application precedence, all validation flavours,
  null-preservation, derivation): two Applications pass byte-identical against the ytt baseline, with
  the two engine-level frictions documented above. The harness is the template for the per-app
  byte-identical migration gate.
