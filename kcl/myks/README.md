# myks KCL schema package

Schemas for the myks KCL configuration layer ([design](../../docs/redesign/design.md)):

- `Environment` — one node of the environment inheritance tree
- `App` — resolved application configuration; the base for prototype schemas
- `finalize` — leaf finalizer; validates leaf invariants (non-empty `id`)
- `SCHEMA_VERSION` — stamped into the frozen tree as `myksSchemaVersion`; the myks engine
  asserts compatibility (same major.minor) at eval time

## Usage

```sh
kcl mod add oci://ghcr.io/mykso/myks
```

```kcl
import myks

env = myks.finalize(parent.env | {
    id = "shop-prod-east"
})
```

Published to `oci://ghcr.io/mykso/myks` by CI on changes to this directory
(`.github/workflows/flow-kcl-package.yml`). Bump `version` in `kcl.mod` together with
`SCHEMA_VERSION` in `version.k` and `supportedKclSchemaVersion` in the engine.
