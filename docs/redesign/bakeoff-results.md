# Config-language bake-off — results

Preserved summary of the bake-off that settled the configuration language (ADR 0002) and the
user-surface model (ADR 0004). The spike code itself has been removed from the repo after the
decisions closed; this document is the durable record. Judged against
[requirements.md](requirements.md).

## Setup

Five tracks implemented the same configuration layer — a realistic 4-level inheritance tree
(root → cluster-group → tier → leaf environment) with applications exercising requirements
L1–L8 — one track per candidate: **KCL (flat layout)**, **KCL (per-app layout)**, **CUE**,
**Jsonnet**, **Pkl**. Correctness gate: each track's resolved configuration tree had to match a
shared golden tree after canonicalization (`yq sort_keys`). All five passed the gate; the winner
was decided by the UX rubric.

## Scorecard

| Dimension | w | KCL (flat) | KCL (per-app) | CUE | Jsonnet | Pkl |
|---|---|---|---|---|---|---|
| User-surface readability | ×2 | 4 | 4 | 4 | 4 | 4 |
| LSP / editor support (L6) | ×2 | 4 | 4 | 3 | 2 | 4 |
| Eval speed, benchmarked (L8) | ×1.5 | 5 | 5 | 5 | 5 | 2 |
| Error-message quality | ×1.5 | 4 | 4 | 4 | 3 | 5 |
| Typing & validation (L7) | ×1 | 5 | 5 | 5 | 2 | 5 |
| Library / reuse (L5) | ×1 | 4 | 4 | 4 | 4 | 4 |
| Learning curve | ×1 | 3 | 3 | 2 | 4 | 3 |
| Engine-harness complexity | ×0.5 | 5 | 5 | 5 | 5 | 5 |
| **Weighted total** (max 52.5) | | **44.0** | **44.0** | **41.0** | **36.5** | **41.0** |

Eval speed (whole tree, warm): KCL ~30–66 ms, CUE ~10 ms, Jsonnet ~10–15 ms, Pkl ~0.7 s
(JVM-startup-bound). LSP scores verified hands-on in an editor over the spike tree: all five
language servers work as documented.

## Synthesis

**Winner: KCL** — leads the weighted rubric with no dimension below 3; the only candidate pairing
full static typing (L7=5) with fast eval (L8=5) and a shipping language server.

Runners-up, both genuinely close:

- **CUE (41.0)** — lightest file-layout story (package auto-aggregation: zero import/roster
  wiring) and no derived-field guard tax. Cost is conceptual: unification-not-mutation is a real
  learning-curve hit, and myks's core operation — last-wins override down the tree — must be
  re-modeled per field (two layers setting the same scalar concretely is a hard *conflict* in CUE,
  not an override).
- **Pkl (41.0)** — best error messages in the field, full typing, glob-import discovery. Only real
  weakness: JVM startup (~0.7 s warm) — startup-bound, not tree-bound.

**Last: Jsonnet (36.5)** — gentlest learning curve, but the typing gap is structural (no types, no
closed structs, no required fields; validation is hand-written runtime `assert`) and merge
silently *replaces* arrays (forget `parent.apps +` → inherited apps vanish, no error). Not
recommended for a config layer whose job is catching misconfig before render.

## Flat vs per-app layout — settled by the scale spike

An 11-cluster A/B (both flavours passing the same golden gate) plus a generated-tree benchmark up
to 1000 clusters settled the tie:

- **Flat is the default, per-app is a per-app opt-in.** 20 of 25 files were byte-identical between
  flavours: at realistic scale the tree is dominated by roster + identity levels where the layouts
  don't differ. Per-app buys a guessable path (`<env>/<app>/app.k`) but charges mandatory
  indirection — a one-value per-cluster override costs +5 lines/1 file flat vs +9 lines/2 files/1
  dir per-app — and KCL has no subpackage auto-discovery, so the wiring never leaves `env.k`.
  Nothing forces a global choice: both styles coexist in one tree, so big apps can take a
  directory while everything else stays flat.
- **Scale numbers (whole-tree eval, generated trees):** linear, no knee — ~40 ms fixed startup +
  ~1.6 ms/cluster, ~0.9 MB RSS/cluster, measured to 1000 clusters. Single-env eval stays constant
  (~55–60 ms) regardless of tree size. Per-app layout costs a parse-only ~10–20% at 200+ clusters,
  nothing at realistic size. These numbers accepted ADR 0003.

## Key KCL findings (carried into [design.md](design.md))

Wins:

- **Language-native inheritance is clean.** Each level is `parent.env { overrides; apps += [...] }`:
  last-wins scalars, `+=` list append, nested-path override all native. No hand-written deep-merge
  — that tax was an artifact of the earlier file-folding model and vanishes with in-language
  inheritance.
- **Import boilerplate is light:** ~2–3 lines per level (one import per parent + per lib),
  absolute from module root. Directory tree == package tree.
- **L2/L3/L4 fall out in one eval scope:** self-reference = schema default over a sibling attr;
  cross-stage = conditional dict-union in a schema default; cross-app = comprehensions over the
  assembled roster.
- **Typing/validation is full:** schemas, unions, `check` blocks, nullable, conditional
  cross-field — all verified to fire on bad input.
- A shared `finalize` lambda + derived identity collapses a plain leaf env to ~3 substantive
  lines.

Papercuts (all workable, none a blocker):

- **KCL re-evaluates derived defaults and `check` on every config override**, so eager derivations
  must tolerate not-yet-set intermediate state (guard: `... if region in _regionShort else ""`).
- **List-element patching hits the static-type wall:** `a {field = ...}` on a comprehension
  variable typed by the list fails for subtype fields; bind the patch to a dict and union it
  (`a | _patch`) instead.
- **Package paths must be identifiers:** `central_forwarder/`, not `central-forwarder/`; display
  names live in the config body. Hyphenated keys need explicit nesting, not dotted shorthand.
- Config-override doesn't parse on an index expression (`_byName["k"] {…}`) — bind to a var first.
- `sorted()` takes no key function (scalars only).
- Imports traverse directories with no `.k` files, and a child package can import its ancestor —
  variable-depth trees need no stub files.
