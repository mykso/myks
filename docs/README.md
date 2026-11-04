# Myks documentation

Myks renders the Kubernetes manifests of many clusters from one Git repository.
New here? Read [Why myks](why-myks.md), then follow
[Getting started](getting-started.md).

## Start here

- [Why myks](why-myks.md): the rendered manifests workflow, and how myks
  compares with Helm, Kustomize, Helmfile and others.
- [Installation](installation.md): Homebrew, Nix, Docker, packages, CI.
- [Getting started](getting-started.md): a ten-minute tutorial with two
  clusters.
- [Concepts](concepts.md): prototypes, applications, environments, inheritance
  and the render pipeline.

## Guides

- [Data values](data-values.md): configure applications and environments, and
  add your own values.
- [Sources](sources.md): download charts and manifests from Helm, OCI, Git,
  GitHub releases and private repositories.
- [Cookbook](cookbook.md): recipes for common tasks.
- [GitOps and CI](gitops-ci.md): render in pull requests, validate the output,
  and catch drift.
- [Optimizations](optimizations.md): parallelism, caching and selective
  rendering.
- [Migrating to KCL](migration.md): convert a ytt project to KCL mode.

## Reference

- [CLI](cli.md): commands, arguments, flags and environment variables.
- [Configuration](configuration.md): `.myks.yaml` and naming conventions.
- Render steps: [Helm](helm.md), [ytt](ytt.md),
  [ytt libraries](ytt-libraries.md), [Static files](static-files.md),
  [kbld](kbld.md), [Argo CD](argocd.md).
- [Smart Mode](smart-mode.md): which changes select which environments and
  applications.
- [Plugins](plugins.md): extend myks with your own commands.
- [Inspecting and debugging](inspect.md): see which files and values an
  application uses.
- [KCL mode](kcl-mode.md): the typed configuration layer (preview).

## Project

- [Contributing](../CONTRIBUTING.md): build, test, and release.
- [Changelog](../CHANGELOG.md).
- [Why KCL](why-kcl.md): how the new configuration language was chosen.
- Configuration-layer redesign: [requirements](redesign/requirements.md),
  [research findings](redesign/research-findings.md),
  [bake-off results](redesign/bakeoff-results.md), [design](redesign/design.md),
  [roadmap](redesign/roadmap.md).
- Architecture decision records:
  [0001](adr/0001-evolve-myks-not-adopt-foreign-tool.md) build on myks,
  [0002](adr/0002-replace-ytt-data-values-with-kcl.md) KCL for data values,
  [0003](adr/0003-evaluate-whole-tree-then-fan-out.md) whole-tree evaluation,
  [0004](adr/0004-pure-language-config-surface.md) pure-language surface,
  [0005](adr/0005-go-host-evolve-in-place.md) evolve in place in Go,
  [0006](adr/0006-kcl-engine-integration.md) KCL engine integration,
  [0007](adr/0007-prototype-bases-as-packages.md) prototype bases as packages.
- [Running embedded tools in-process](in-process-embedded-tools/README.md): an
  investigation.
- [Notes for AI agents](agents/domain.md).
