# Rendering Redesign — Research Findings

Evidence base for the rendering-pipeline redesign. Distilled from a 2025-2026 landscape survey
(config languages, integrated GitOps tools, pluggable-pipeline prior art, myks fork feasibility).
Decisions that flow from this live in `docs/adr/`; vocabulary in `/CONTEXT.md`.

## Baseline we must match (verified against this repo)

61 prototypes · 25 leaf environments · ~681 ArgoCD Applications · ~22.5k rendered per-resource YAML files ·
1,030 `.ytt.yaml` + 9 `.star` modules · 9 kbld mirror rules · `async: 16` · vendir cache in `.myks/vendir-cache/` ·
3 plugins (rightsize/karl, argo-refresh, debug). Pattern = **Rendered Manifests Pattern** (render in CI → commit
hydrated YAML → ArgoCD applies static YAML). myks itself: independent, ~17 stars → **low bus factor is the real
driver to evaluate alternatives**.

### The 8 substrate capabilities (any replacement must match)
(a) fetch helm/git/http  (b) cross-run cache  (c) parallel render  (d) tree inheritance + reuse
(e) per-resource sliced YAML → git  (f) ArgoCD Application generation  (g) image-ref mirroring (kbld)
(h) unified multi-engine rendering-plugin layer

## Finding 1 — No existing tool covers all of (a)–(h)

Every wholesale replacement forces rebuilding 2-4 substrate capabilities + glue.

| Tool | a | b | c | d | e | f | g | h | Lang + LSP | Migration |
|---|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|---|:-:|
| **myks (today)** | ● | ● | ● | ● | ● | ● | ● | ● | ytt — weak LSP | — |
| Holos | ◐ | ● | ● | ● | ◐ | ● | ○ | ◐ | CUE — alpha LSP | MED-HIGH |
| Helmfile | ● | ● | ● | ● | ● | ◐ | ◐ | ◐ | Go-tmpl — none | MED-HIGH |
| cdk8s | ◐ | ○ | ○ | ◐ | ● | ◐ | ○ | ○ | TS/Py/Go — strong | HIGH |
| KCL eco | ◐ | ◐ | ○ | ● | ◐ | ● | ◐ | ○ | KCL — good LSP | HIGH |
| kpt+KRM | ◐ | ○ | ○ | ◐ | ◐ | ○ | ● | ◐ | Kptfile — none | HIGH |
| Kustomize | ● | ○ | ○ | ● | ◐ | ○ | ● | ◐ | YAML — none | HIGH |
| Timoni | ◐ | ● | ○ | ◐ | ○ | ○ | ○ | ○ | CUE — alpha | HIGH |
| kluctl | ● | ◐ | ● | ● | ◐ | ○ | ◐ | ○ | Jinja2 — none | HIGH |
| ArgoCD CMP | ● | ○ | n/a | n/a | ○* | ◐ | n/a | ●(shell) | wrapper | LOW |

(● native · ◐ partial/build-on-top · ○ none. *CMP renders live at sync, not to git.)

**Closest twins:** Holos (right architecture: render-to-git + native ArgoCD-app gen + CUE inheritance +
generator→transformer pipeline; but no image mirroring, no auto per-resource slicing, ~160 stars/1 company) and
Helmfile (fetch/cache/parallel/env good; mirroring/slicing/argocd-gen become DIY). Both downgrade ≥2 hard reqs.

## Finding 2 — myks is already ~80% a plugin engine; forking is low-risk

- `YamlTemplatingTool` interface (`internal/myks/plugins.go:18-23`) is already the render-step contract:
  `Render(prevFile) (out,err)` · `Ident()` · `IsAdditive()` · `AcquireLock()`.
- Pipeline order is ONE hardcoded slice literal: `internal/myks/globe.go:276-282`.
- The driver loop (`internal/myks/render.go:43-72`) already implements additive (`\n---\n` concat) vs mutative
  (replace) generically, and writes numbered step artifacts.
- Gaps: `static` + `argocd` are hardcoded tail calls (`globe.go:287-294`); `slice` fused into `RenderAndSlice`;
  `sync` (vendir/helm) is a separate hardcoded phase (maintainers already TODO'd extraction).
- myks already vendors ytt/vendir/kbld as Go libs but invokes them by re-execing itself (`process.go` `myksFullPath()`).

**Effort:** (i) config-driven ordered in-process plugin registry = **LOW/LOW** (move slice literal → registry+factory,
promote static/argocd into the interface). (ii) allow plugin = in-process Go OR external exec via KRM = **MED/MED**
(bounded adapter; main risk = unifying with existing post-render `Plugin` + keeping output byte-identical).

## Finding 3 — KRM ResourceList is the cross-tool plugin wire standard

`apiVersion: config.kubernetes.io/v1, kind: ResourceList` with `items` (resources in/out) + `functionConfig`
(args) + `results` (diagnostics). stdin→stdout, stderr=logs, exit≠0=fail. Implemented by **kpt AND kustomize**
(two independent hosts) + Go/Starlark/KCL SDKs. exec (local binary) vs container (OCI, hermetic, digest-pinnable)
share one wire format. kpt rebooted by Nephio community Mar 2025 (active, perpetually beta).
**Use as the EXTERNAL wire contract only** — don't force in-process Go steps through ResourceList serialization.

## Finding 4 — Config language ranking (replace ytt data-values)

Weighted for the stated priorities: editor tooling/LSP (the #1 user pain) ×2, Go-embeddability ×1.5.

| Rank | Lang | Tooling | Expressive | Go-embed | Ecosystem | Learning | Note |
|---|---|:-:|:-:|:-:|:-:|:-:|---|
| 1 | **KCL** | 4 | 4 | 3.5(cgo) | 3.5 | **5** | Python-like, gentlest on-ramp; LSP across VSCode/JetBrains/Neovim; cgo Go SDK; CNCF activity cooled |
| 2 | **CUE** | 4 | 5 | **5** pure-Go | 4 | 2 | Best embed + correctness; $10M-backed 2025; but HARDER than ytt, no `else`, import-unify ≠ overlay |
| 3 | Jsonnet | 3.5 | 4.5 | **5** pure-Go | 4 | 2.5 | go-jsonnet + Tanka helm-overlay proven; but NO types/schema (regression) |
| 4 | cdk8s | **5** | 5 | 2.5(jsii) | 3.5 | 2.5 | best IDE; but real-language toolchain + jsii-for-Go overhead |
| — | Pkl | 3.5 | 4.5 | 2(subproc) | 3 | 3 | no in-proc Go, no helm-overlay story |
| — | Nickel/Starlark/Dhall | low | — | — | low | — | immature typed tooling / stagnant — avoid |

**Bottom line:** KCL = best fit for the *team* (kills the comprehension+LSP pain). CUE = best fit for the *tool*
(pure-Go embed, correctness) but raises cognitive load — wrong if reducing it is the goal. Jsonnet = lowest
integration risk (pure-Go) but no type system.

## Emergent recommended shape

Build on the myks lineage (see ADR `docs/adr/0001`) → keep all substrate → turn the fixed pipeline into a
**config-driven plugin registry** (internal Go-style step interface) → adopt **KRM ResourceList** for external/exec
plugins → replace the **ytt data-values engine** with a real language → migrate config app-by-app behind a
**byte-identical-output** gate.

### Decisions status
- **DECIDED** — do not adopt a foreign tool; build on myks (ADR 0001). Adopt+glue is rejected.
- **DECIDED** — rendering = unified plugin engine; KRM ResourceList = external wire contract.
- **OPEN, session 1 (handed off)** — configuration language to replace ytt data-values (KCL vs CUE vs Jsonnet vs
  cdk8s). Handoff brief: `$TMPDIR/handoff-config-language-selection.md`.
- **OPEN, session 2** — host programming language + fork-myks vs rewrite-from-scratch.
