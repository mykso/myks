# Why KCL

myks replaced ytt's data-values composition with
[KCL](https://www.kcl-lang.io/). This page explains why KCL was chosen over the
other candidates. ytt stays for match-and-mutate overlays — the new language
computes values, ytt still patches rendered output.

The full decision record is
[ADR 0002](adr/0002-replace-ytt-data-values-with-kcl.md); the raw scorecard and
findings are in [bakeoff-results.md](redesign/bakeoff-results.md).

## What the language has to do

myks collects the data values on an Application's inheritance path (root →
cluster-group → tier → leaf → prototype) and merges them into one struct. The
operation to reproduce is: **last-wins override down the tree, list append,
typing and validation, and derived values** — the last one being what ytt data
values cannot do (no field can derive from another), and the main reason to
replace it. Weak editor support for ytt was the other main pain.

## How it was decided

Five tracks implemented the same configuration layer — a realistic 4-level
inheritance tree with applications exercising every requirement in
[requirements.md](redesign/requirements.md): **KCL (flat layout)**, **KCL
(per-app layout)**, **CUE**, **Jsonnet** and **Pkl**. Each track's resolved tree
had to match a shared golden tree. All five passed, so correctness did not
separate them; the winner was picked on a weighted UX rubric:

| Dimension                     | w    | KCL (flat) | KCL (per-app) | CUE      | Jsonnet  | Pkl      |
| ----------------------------- | ---- | ---------- | ------------- | -------- | -------- | -------- |
| User-surface readability      | ×2   | 4          | 4             | 4        | 4        | 4        |
| LSP / editor support (L6)     | ×2   | 4          | 4             | 3        | 2        | 4        |
| Eval speed, benchmarked (L8)  | ×1.5 | 5          | 5             | 5        | 5        | 2        |
| Error-message quality         | ×1.5 | 4          | 4             | 4        | 3        | 5        |
| Typing & validation (L7)      | ×1   | 5          | 5             | 5        | 2        | 5        |
| Library / reuse (L5)          | ×1   | 4          | 4             | 4        | 4        | 4        |
| Learning curve                | ×1   | 3          | 3             | 2        | 4        | 3        |
| Engine-harness complexity     | ×0.5 | 5          | 5             | 5        | 5        | 5        |
| **Weighted total** (max 52.5) |      | **44.0**   | **44.0**      | **41.0** | **36.5** | **41.0** |

Eval speed (whole tree, warm): KCL ~30–66 ms, CUE ~10 ms, Jsonnet ~10–15 ms, Pkl
~0.7 s (JVM-startup-bound). All five language servers were checked hands-on in
an editor.

## Why KCL

KCL is the only candidate that pairs full static typing with fast evaluation and
a working language server, and it scores at least 3 on every dimension.

- **Inheritance is native.** Each level is
  `parent.env { overrides; apps += [...] }`: last-wins scalars, `+=` list append
  and nested-path override need no hand-written merge.
- **Derived values, cross-stage and cross-app values** fall out of one
  evaluation scope — schema defaults over sibling attributes, comprehensions
  over the assembled application roster.
- **Full typing and validation:** schemas, unions, `check` blocks, nullable and
  conditional cross-field rules.
- **Familiar syntax.** KCL is Python-shaped, and ytt users already write
  Starlark: the two share literals, operators and comprehensions.

## Why not CUE

CUE was a close second (41.0) and has real strengths: a pure-Go embed, the best
correctness model, package auto-aggregation (no import wiring between files) and
no guard tax on derived fields. It lost on fit with myks's core operation and on
learning curve:

- **CUE unifies; it does not override.** Two layers setting the same scalar is a
  hard error (`conflicting values "rke2" and "gke"`), not an override. To make
  layering work, every overridable field must be re-modelled as a
  `*default | type` disjunction — a change spread across every user-written
  configuration file. Any field that two layers both set concretely stays a
  footgun.
- **Steeper learning curve** (scored 2, the lowest): unification instead of
  mutation is a new mental model, and there is no `else`. The research survey
  rated it harder than ytt — the wrong direction when reducing cognitive load is
  the goal.
- **Weaker editor support** (scored 3 vs 4, on a ×2 dimension).

CUE's pure-Go embed was the one argument that could have flipped the choice:
KCL's Go SDK loads a native library. That concern is resolved — the SDK loads it
via purego (no CGO), covers all release platforms and runs in-process (see
[ADR 0006](adr/0006-kcl-engine-integration.md)).

## Why not Pkl

Pkl tied CUE at 41.0, with the best error messages in the field, full typing and
glob-import discovery. Its one real weakness is speed: evaluation is bound by
JVM startup (~0.7 s warm), which myks would pay on every render.

## Why not Jsonnet

Jsonnet (36.5) has the gentlest learning curve but is not recommended for a
layer whose job is to catch misconfiguration before render:

- **No type system:** no types, closed structs or required fields; validation is
  hand-written runtime `assert`.
- **Merging silently replaces arrays:** forgetting `parent.apps +` makes
  inherited applications vanish, with no error.

## Not shortlisted

- **cdk8s** — the best IDE support, but it brings a full programming-language
  toolchain and jsii overhead for Go.
- **Nickel, Starlark, Dhall** — immature typed tooling or a stagnant ecosystem.
- **Adopting another tool wholesale** (Holos, Timoni, Helmfile, kluctl and
  others) was ruled out before the language choice: none covers everything myks
  does, see [ADR 0001](adr/0001-evolve-myks-not-adopt-foreign-tool.md) and
  [research-findings.md](redesign/research-findings.md).

## Costs of KCL

Known papercuts, all workable:

- KCL re-evaluates derived defaults and `check` blocks on every override, so
  derivations must tolerate not-yet-set intermediate values.
- Package paths must be identifiers: `central_forwarder/`, not
  `central-forwarder/`.
- `sorted()` takes no key function.

The full list is in [bakeoff-results.md](redesign/bakeoff-results.md). If KCL
ever has to be replaced, CUE and Pkl are the near-tie fallbacks.
