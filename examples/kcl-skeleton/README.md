# KCL walking-skeleton example

Minimal repo exercising the KCL-based configuration layer
([design](../../docs/redesign/design.md)). The `kcl.mod` file at the root
selects KCL mode: myks evaluates the KCL module into a frozen resolved tree and
discovers environments and applications from it — no filesystem walk, no ytt
data-values authoring.

- `main.k` emits the frozen tree (`myksSchemaVersion`, `environments`)
- `envs/env.k` holds global defaults and the shared application roster
- `envs/dev/env.k` and `envs/prod/env.k` import the base level and patch it
- `prototypes/hello` is a regular myks prototype with an inline (hermetic)
  helm chart; per-app resolved values from the tree reach ytt templates via a
  generated data-values file

## Run

```sh
myks render ALL
```

Smart Mode is not supported in KCL mode: a plain `myks render` warns and
renders everything.

## File tree

```python
.
├── kcl.mod                # KCL module manifest; the KCL-mode marker
├── main.k                 # root evaluation: emits the frozen resolved tree
├── envs
│   ├── env.k              # base level: defaults + application roster
│   ├── dev
│   │   └── env.k          # leaf environment: imports the base, patches values
│   └── prod
│       └── env.k
├── prototypes
│   └── hello
│       ├── helm
│       │   └── 'hello.ytt.yaml'  # helm values computed from resolved app config
│       └── vendir
│           └── 'base.ytt.yaml'   # inline helm chart (no network needed)
└── rendered               # rendered manifests for all environments
```
