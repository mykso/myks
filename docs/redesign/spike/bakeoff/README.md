# Config-language bake-off

Picks the replacement for ytt data-values composition (reopens ADR 0002) by implementing the **full**
config layer in each candidate language, then scoring on UX. Scope, requirements, rubric:
`../../requirements.md`. Eval model: `../../../adr/0003-evaluate-whole-tree-then-fan-out.md`.

Not an extension of `../tree/` (that stays a per-app KCL *semantics* reference). This builds the
**whole resolved env tree** at the **pure-language user surface** — env levels + prototypes + apps
authored in the candidate language, one import graph, one-pass eval.

```
fixtures/golden.yaml    independent oracle: expected resolved tree for env tools-stage-eu-dc1
fixtures/scenarios.md   concrete app ↔ requirement matrix
compare.sh              the gate: emit | canonicalize | diff vs golden
kcl/                    KCL candidate — full tree, flat leaf assembler (main.k)   [PASS, baseline]
kcl-apps/ cue/ jsonnet/ pkl/   todo — see HANDOFF.md
SCORECARD.md            rubric scores + findings per language (+ file-layout concern)
HANDOFF.md              fan-out plan: 4 parallel tracks, shared gate + constraints
```

## Run

```shell
cd docs/redesign/spike/bakeoff
nix shell nixpkgs#kcl nixpkgs#yq-go -c bash compare.sh 'kcl run kcl/'
# -> PASS — resolved tree matches golden
```

The gate canonicalizes both sides (`yq -P 'sort_keys(..)'` + comment-strip): PASS means the **data**
is identical, not raw bytes (tools quote/order differently — cosmetic).
