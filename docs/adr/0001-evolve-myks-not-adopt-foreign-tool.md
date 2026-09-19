# Build on the myks lineage, do not adopt a foreign rendering tool

We will redesign the GitOps renderer by **building on myks** (fork-and-evolve, or rewrite sharing its model —
that sub-decision was later settled: evolve in place, ADR 0005) rather than adopting an existing integrated tool
(Holos, Helmfile, kluctl, Timoni, cdk8s, KCL ecosystem, kpt/kustomize) as a wholesale replacement.

## Why

A 2025-2026 landscape survey (see `docs/redesign/research-findings.md`) found **no existing tool natively covers
all 8 substrate capabilities** we rely on (source fetch, cross-run cache, parallel render, tree inheritance,
per-resource sliced YAML→git, ArgoCD Application generation, image-ref mirroring, unified rendering-plugin layer).
Every candidate forces rebuilding 2-4 of them plus glue. The two closest twins each downgrade ≥2 hard requirements:
Holos has no image mirroring / no automatic per-resource slicing and ~160 stars / single-company bus factor;
Helmfile turns mirroring, per-resource slicing and ArgoCD-app generation into DIY work. Meanwhile myks is small, MIT,
and **already ~80% a plugin engine** (clean `YamlTemplatingTool` interface; pipeline order is a single hardcoded
slice at `globe.go:276`; the additive/mutative loop is generic), so evolving it is low effort / low risk.

## Consequences

- Sub-decisions sequenced as separate sessions: (1) **configuration language** to replace ytt data-values
  (decided: KCL — ADR 0002, confirmed by the bake-off in `docs/redesign/bakeoff-results.md`),
  then (2) **host programming language** and **fork-vs-rewrite** (decided: Go, evolve in place — ADR 0005).
- We will adopt the **KRM ResourceList** spec as the external/exec rendering-plugin wire contract (the one cross-tool
  standard), while keeping an internal Go-style step interface for in-process renderers.
- We own renderer maintenance (already true today), accepting that in exchange for exact fit to our model and an
  incremental, byte-identical-output migration path.
