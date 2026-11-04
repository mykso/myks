---
title: KCL mode (preview)
sidebar:
  order: 15
---

KCL mode replaces the ytt data values files (`env-data*.yaml`, `app-data*.yaml`)
with a configuration tree written in [KCL], a typed configuration language with
a Python-like syntax and editor support. Everything else stays: prototypes,
`vendir/`, `helm/`, `ytt/`, `static/` and `argocd/` directories, overlays, the
render pipeline and the output.

:::caution

KCL mode is a preview. It is complete enough to render real repositories
byte-identically to their ytt setup, but Smart Mode, `myks init` and value
provenance do not support it yet, and the schema package can still change before
the next major release.

:::

## Why

ytt data values are computed in one pass, so a value cannot derive from another
value. Real repositories work around that with values set twice and keys
hard-coded across prototypes. KCL evaluates the whole tree at once: a value can
derive from any other value of its application, its environment, or a sibling
application, and schemas catch mistakes with types and checks. See
[Why KCL](https://github.com/mykso/myks/blob/main/docs/why-kcl.md) for how KCL
was chosen over CUE, Pkl and Jsonnet.

## How it works

A `kcl.mod` file at the project root switches the project to KCL mode. Myks then
evaluates `main.k` once, before any sync or render, into a resolved tree of
every environment and application. Environments and applications are discovered
from that tree, not from the filesystem. Each application's resolved values are
handed to the existing pipeline as generated ytt data values, so templates keep
reading `data.values.*` unchanged.

```text
.
├── kcl.mod                 # selects KCL mode, pins the myks schema package
├── main.k                  # lists every environment
├── envs/
│   ├── env.k               # root level: defaults, shared applications
│   └── prod/
│       ├── env.k           # imports the parent level and patches it
│       └── eu/
│           ├── env.k       # a leaf: id and finalize
│           └── app-frontend.k
└── prototypes/
    └── webapp/
        ├── proto.k         # prototype schema with defaults
        ├── helm/ ...       # unchanged
        └── vendir/ ...     # unchanged
```

Each level imports its parent and overrides it:

```kcl
# envs/env.k
import myks

env = myks.Environment {
    applications.hello = myks.App {
        proto = "hello"
        application.greeting = "hello"
    }
}
```

```kcl
# envs/dev/env.k
import myks
import envs

env = myks.finalize(envs.env | {
    id = "kcl-dev"
    applications.hello.application.greeting = "hello from dev"
    applications.hello.helm.namespace = "dev-hello"
})
```

```kcl
# main.k
import myks
import envs.dev

myksSchemaVersion = myks.SCHEMA_VERSION
environments = {
    dev = dev.env
}
```

The
[kcl-skeleton](https://github.com/mykso/myks/blob/main/examples/kcl-skeleton/README.md)
example is a complete minimal project;
[kcl-integration-tests](https://github.com/mykso/myks/blob/main/examples/kcl-integration-tests/README.md)
exercises every render step.

The `myks` schema package (`Environment`, `App`, `Apps`, `with_apps`,
`finalize`) is published to `oci://ghcr.io/mykso/myks`; see its
[README](https://github.com/mykso/myks/blob/main/kcl/myks/README.md). Myks
checks at evaluation time that the project's schema version matches the one it
supports.

## Getting started

- **Existing ytt project:** run `myks migrate`. It writes the KCL tree next to
  the ytt files and renders byte-identically. Then hand-finish it. Follow the
  [migration guide](../../guides/migration/).
- **New project:** copy
  [examples/kcl-skeleton](https://github.com/mykso/myks/tree/main/examples/kcl-skeleton/)
  and pin the published schema package:

  ```shell
  kcl mod add oci://ghcr.io/mykso/myks
  ```

Delete `kcl.mod` to switch a migrated project back to its ytt files.

## Current limitations

- **No Smart Mode.** `myks render` without arguments warns and renders every
  environment.
- **No scaffold.** `myks init` creates ytt projects only.
- **No data value provenance.** `myks inspect apps --provenance` and
  `--redundant` only cover Helm and ytt-pkg values files.
- **Identifiers only.** Directory names under `envs/`, and prototype directories
  with a schema, must be KCL identifiers: `cert_manager`, not `cert-manager`.
  Application names are free.
- **Native library.** The KCL runtime extracts a native library to a cache
  directory on first use. On read-only filesystems, point `KCL_LIB_HOME` at a
  writable directory.

The design and its decisions are in
[redesign/design.md](https://github.com/mykso/myks/blob/main/docs/redesign/design.md)
and the [ADRs](https://github.com/mykso/myks/tree/main/docs/adr/).

[KCL]: https://www.kcl-lang.io/
