# GitOps Rendering Context

Shared vocabulary for the manifest-rendering pipeline (myks + vendir/helm/ytt/kbld → ArgoCD)
and its planned redesign. Glossary only — no implementation detail, no decisions (those live in `docs/adr/`).

## Myks entities

**Environment**:
A leaf node in the env-data inheritance tree that has `environment.id` set; corresponds 1:1 to a Kubernetes cluster.
Only leaves render output; intermediate nodes (cluster group, tier) exist purely to carry inherited configuration.
_Avoid_: cluster (use only for the physical thing), env-group (that's an intermediate node).

**Application**:
A set of manifests deployed together as one ArgoCD Application. Rendered in isolation from sibling applications.
_Avoid_: app (in prose), service, workload.

**Prototype**:
A reusable template an Application is instantiated from. Carries source definitions (vendir), helm values, and overlays.
Conceptually part of the Application, not a separate deployable thing.
_Avoid_: template, base, chart.

## Configuration model

**Data values**:
The single flat YAML struct ytt computes for an Application by merging every discovered `*-data.*.yaml` file.
Computed in one pass *before* templating; therefore **cannot self-reference and carries no conditional or
computed logic** — a value can never derive from another value. This single property is the root cause of the
cross-referencing and double-toggle pain.
_Avoid_: config, values (unqualified), params.

**Prototype scope** (`prototype.*`) vs **Application scope** (`application.*`):
The convention that splits data values by who reads them. Vendir (the sync stage) reads only `prototype.*`; application
config lives under `application.*`. Because data values cannot self-reference, an `application.*` toggle cannot switch a
`prototype.*` source on/off — forcing the **double-toggle** (set the flag in both places by hand).
_Avoid_: app-data vs env-data (those are file-suffix conventions, a different axis).

**Shared key**:
A flag hoisted to environment or global scope (e.g. `runsGrafanaOperator`, `centralForwarder.*`) so that multiple
Applications can read it by a hardcoded path. The standard workaround for the no-self-reference limitation; fragile
under refactoring because the path is a stringly-typed contract spread across prototypes.
_Avoid_: global flag, feature flag.

## Pipeline stages

**Sync stage**:
The pre-render phase that fetches sources — vendir downloads external artifacts, helm builds charts. Runs before any
templating. Its vendir config is itself rendered by ytt from the `prototype.*` data values.

**Render stage**:
The fixed, hardcoded sequence helm-template → ytt-pkg → ytt-overlay → global-ytt → kbld → slice → static → argocd.
Order is baked into myks `processApp()`; not pluggable without forking.
_Avoid_: build, compile.

**Additive vs mutative step**:
An additive step (helm, vendir) produces new output; a mutative step (ytt overlay, global-ytt, kbld) transforms the
previous step's output. The render stage interleaves both.

**Value computation vs match-mutate overlay**:
The two halves of ytt's templating work, separated by the `compute │ patch` seam. _Value computation_ = deriving the
data-values struct and templating helm values / generating resources from it (where self-reference, conditionals and
computed logic are wanted). _Match-mutate overlay_ = patching already-rendered helm output by matching nodes and
mutating them (`global-ytt`, prototype `ytt/` overlays). The redesign assigns value computation to the new config
language and keeps match-mutate overlay in ytt (see `docs/adr/0002`).
_Avoid_: patch (use only for the mutate half), templating (ambiguous across the seam).

**Plugin (myks)**:
An external executable named `myks-*` invoked *after* rendering, receiving context via env vars + `MYKS_DATA_VALUES`.
Cannot inject data values, add render steps, or hook sync. (e.g. rightsize, argo-refresh.)
_Avoid_: extension, hook (reserve for a future in-pipeline concept).
