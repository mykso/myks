# Myks documentation

The user documentation is published at **https://myks.dev/**. Its pages live in
[`src/content/docs/`](src/content/docs/), and [`site.yaml`](site.yaml) holds the
landing page. Build the site locally with `npm ci && npx astro build` from this
directory; the [docs-kit](https://github.com/Zebradil/docs-kit) package does the
rest.

The rest of this directory is about developing myks, not using it:

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
- [Contributing](../CONTRIBUTING.md) and the [Changelog](../CHANGELOG.md).
