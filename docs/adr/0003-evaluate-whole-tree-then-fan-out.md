---
status: proposed
---

# Evaluate the whole configuration tree once, then fan out rendering

We will compute the **entire** configuration tree (root → cluster-group → tier → region → every
environment → every application) in **one evaluation pass** into a single frozen, fully-resolved
structure, then **fan out** per-application sync + render in parallel reading from that frozen result.
This replaces today's model where each Application's data values are composed in isolation.

## Why

The redesign requires **cross-application introspection within an environment** (requirements L4):
app B targets app A's namespace; alertmanager routes by every app's namespace + metadata; grafana
dashboards render iff grafana-operator is in the env roster. ytt fakes this with hand-hoisted
**shared keys** because data values are composed per-app with no sibling visibility.

A single full-tree evaluation makes every cross-reference (inheritance, self-reference, cross-stage,
cross-application) resolve naturally in one scope — all three candidate languages express this as
ordinary references / comprehensions over one record. The cost (an app's value depending on a
sibling) is paid **once at eval time**, not per-app, so the **fan-out stays a pure parallel read** of
precomputed values — isolation moves from "apps can't see each other" to "render never re-evaluates."

Scoping eval to a subtree buys nothing: inheritance already forces pulling every ancestor level up to
root to evaluate any environment. So eval the whole tree.

## Consequences

- **Two phases:** full-tree evaluation (one pass, all cross-refs resolved) → fan-out (per-app
  parallel sync + render, read-only over the frozen tree). Smart change detection operates on the
  fan-out by diffing the resolved tree, not on the eval.
- **Eval speed becomes a hard gate** (requirement L8). "Eval everything" is only viable if full-tree
  eval is fast; this is benchmarked per language in the bake-off. If too slow across **all**
  candidates, this decision is reconsidered (subtree eval, memoization, or caching the frozen tree).
- **Cross-environment references** are not a use case; the eval computes the whole tree but apps are
  expected to reference only within their environment + ancestors. Not enforced unless a need appears.
- Kills the **shared-key** and **double-toggle** workarounds: scope-2 and scope-3 introspection
  become direct references instead of hand-mirrored hoisted paths.
