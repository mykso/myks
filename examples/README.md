# Examples

Example myks projects, each with its rendered output. They double as integration
test fixtures.

- [simple](./simple/README.md): two environments and one Helm chart, with a
  per-environment chart version and values override.
- [default](./default/README.md): the project `myks init` creates.
- [integration-tests](./integration-tests/README.md): every inheritance level
  and render step, including multiple sources, per-chart Helm settings, static
  files and ytt libraries.
- [kcl-skeleton](./kcl-skeleton/README.md): a minimal project in
  [KCL mode](https://myks.dev/guides/kcl-mode/).
- [kcl-integration-tests](./kcl-integration-tests/README.md): the integration
  tests ported to KCL mode, rendering byte-identically to the ytt version.
