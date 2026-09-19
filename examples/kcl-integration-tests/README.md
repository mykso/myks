# Integration tests — KCL mode

This is the [integration-tests](../integration-tests) fixture ported to the KCL configuration
layer ([design](../../docs/redesign/design.md)). Prototypes, env-level ytt overlays, helm values
templates and lib files are identical to the legacy fixture; all ytt data-values authoring
(`env-data.ytt.yaml`, `app-data.ytt.yaml`) is replaced by the KCL tree:

- `kcl.mod` selects KCL mode and pins the [myks schema package](../../kcl/myks) (by path here)
- `main.k` emits the frozen resolved tree
- `envs/env.k` carries the shared defaults and the application roster with root-level values
- `envs/dev/env.k` is the leaf environment with dev-level overrides and derivations

## The byte-identical gate

`TestKclGate` (internal/integration) renders this repo and the legacy fixture and requires their
`rendered/` trees to be byte-identical (modulo the repo path baked into ArgoCD source paths).
It is the migration gate of the config-layer redesign and the test suite of the future
`myks migrate` converter. When changing either fixture, keep the other in sync.
