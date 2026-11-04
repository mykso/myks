---
title: Data values
sidebar:
  order: 2
---

Data values are the configuration of an application: its chart versions, the
values its templates read, the settings of the render steps. Myks computes one
set of data values per application by merging [ytt data values] files from the
prototype and from every level of the environment tree. Every step of the
pipeline reads the result: vendir configurations, Helm values files, ytt
templates and the Argo CD templates.

:::note

This page describes the ytt data values files that `myks init` scaffolds. The
preview [KCL mode](../../guides/kcl-mode/) replaces them with typed KCL code.

:::

## Files and merge order

Data values come from files named `env-data*.yaml` and `app-data*.yaml` (the
patterns are [configurable](../../guides/configuration/#naming-conventions)).
For an application `frontend` of prototype `webapp` in `envs/prod/eu/`, myks
merges, in this order:

1. the built-in schema of myks;
2. `env-data*.yaml` of every level: `envs/`, `envs/prod/`, `envs/prod/eu/`;
3. `app-data*.yaml` of the prototype: `prototypes/webapp/`;
4. `app-data*.yaml` of `_proto/webapp/` at every level;
5. `app-data*.yaml` of `_apps/frontend/` at every level.

Later files win. All `env-data` files come before all `app-data` files, so an
`app-data` file anywhere overrides an `env-data` value for the same key.

Files inside a step directory are only seen by that step. A data values file in
`vendir/`, for example, sets values for the sync stage only. This is how the
sample prototypes keep chart versions next to the vendir configuration that uses
them.

Each file is a ytt data values document (`#@data/values`) or a schema document
(`#@data/values-schema`). `environment.id` is the exception to inheritance: myks
reads it as plain YAML from the environment's own `env-data` file, so set it
there as a literal:

```yaml
#! envs/prod/eu/env-data.ytt.yaml
#@data/values
---
environment:
  id: prod-eu
  applications:
    - proto: webapp
      name: frontend
```

## Scopes

Top-level keys are scopes. `application` is yours; the others configure myks:

| Scope         | Purpose                                                                                              |
| ------------- | ---------------------------------------------------------------------------------------------------- |
| `application` | Your values. Empty by default.                                                                       |
| `environment` | `id` of the environment and its `applications` list. Set in `env-data` files.                        |
| `argocd`      | Generated Argo CD resources ([Argo CD](../../guides/argocd/)).                                       |
| `helm`        | Helm step settings: namespace, release names, CRDs, Kubernetes version ([Helm](../../guides/helm/)). |
| `kbld`        | Image resolution and rewriting ([kbld](../../guides/kbld/)).                                         |
| `render`      | Output settings (`includeNamespace`).                                                                |
| `yttPkg`      | ytt-pkg step settings ([ytt](../../guides/ytt/#ytt-packages)).                                       |
| `myks`        | Read-only runtime values: Git repository URL and branch, current step and app.                       |

Print the built-in schema with its defaults and comments:

```shell
myks init --print --components schema
```

After a render, the same file is in `.myks/tmp/data-schema.ytt.yaml`.

## Adding your own values

The schema is strict: a `#@data/values` file cannot set a key the schema does
not declare. That catches typos, and it means new keys are declared in a schema
document first. A prototype usually declares its values with defaults in one
schema file:

```yaml
#! prototypes/webapp/app-data.ytt.yaml
#@data/values-schema
---
#@overlay/match-child-defaults missing_ok=True
application:
  image: ghcr.io/stefanprodan/podinfo:6.7.0
  replicas: 1
  ingress:
    enabled: false
    host: ''
```

- `#@data/values-schema` goes above the document separator (`---`) and marks the
  document as a schema.
- `#@overlay/match-child-defaults missing_ok=True` allows the document to add
  keys to the existing schema.

Environments then override the values with plain data values files:

```yaml
#! envs/prod/_apps/frontend/app-data.ytt.yaml
#@data/values
---
application:
  replicas: 3
  ingress:
    enabled: true
    host: shop.example.com
```

Schema documents also validate values, with [ytt schema annotations]:

```yaml
#@data/values-schema
---
#@overlay/match-child-defaults missing_ok=True
application:
  #@schema/validation min_len=1
  host: ''
```

A render then fails for every application that leaves `host` empty, which is how
a prototype demands a value from each environment.

### Lists

In a schema, a list declares the type of its items, and its default is empty. To
give a list a default, declare it in a schema file and set it in a separate
values file:

```yaml
#! prototypes/webapp/app-data.schema.yaml
#@data/values-schema
---
#@overlay/match-child-defaults missing_ok=True
application:
  ports:
    - name: ''
      port: 0
```

```yaml
#! prototypes/webapp/app-data.values.yaml
#@data/values
---
application:
  ports:
    - name: http
      port: 80
```

Files sort by name, so `app-data.schema.yaml` loads before
`app-data.values.yaml`.

When several files set the same list, ytt appends the items. That is how
`environment.applications` grows down the tree. To replace a list instead,
annotate it:

```yaml
#@data/values
---
application:
  #@overlay/replace
  ports:
    - name: https
      port: 443
```

### Removing a value

To remove a map entry set at a higher level, use `#@overlay/remove`:

```yaml
#@data/values
---
application:
  namespaces:
    #@overlay/remove
    legacy-team:
```

## Reading values

Templates load data values from `@ytt:data`:

```yaml
#! prototypes/webapp/helm/podinfo.yaml
#@ load("@ytt:data", "data")
#@ app = data.values.application
---
replicaCount: #@ app.replicas
ingress:
  enabled: #@ app.ingress.enabled
```

Data values files cannot read other data values: ytt computes them all in one
pass. To derive an application value from the environment's values, load the
`@myks:data.lib.yaml` library, which exposes the environment's final data values
as `env_data`:

```yaml
#! envs/_apps/frontend/app-data.ytt.yaml
#@ load("@myks:data.lib.yaml", "env_data")
#@data/values
---
application:
  ingress:
    host: #@ "shop." + env_data.environment.id + ".example.com"
```

See [ytt libraries](../../guides/ytt-libraries/) for this and for sharing
functions between files.

## Sharing values between applications

Values that several applications need, such as the cluster's domain or its cloud
region, go into `env-data` files under a scope of your own. Declare the scope
once at the root; `#@overlay/match missing_ok=True` adds a top-level key the
built-in schema does not have:

```yaml
#! envs/env-data.cluster.yaml
#@data/values-schema
---
#@overlay/match missing_ok=True
cluster:
  domain: ''
  region: ''
```

```yaml
#! envs/prod/eu/env-data.cluster.yaml
#@data/values
---
cluster:
  domain: eu.example.com
  region: europe-west4
```

Every template of every application in `envs/prod/eu/` can then read
`data.values.cluster.domain`.

## Checking the result

```shell
# final data values of one application
myks inspect apps prod/eu frontend --data-values
# which file sets each final value
myks inspect apps prod/eu frontend --provenance
# overrides that set a value to what is already inherited (fails in CI)
myks inspect apps --redundant
```

See [Inspecting and debugging](../../guides/inspect/).

[ytt data values]: https://carvel.dev/ytt/docs/latest/how-to-use-data-values/
[ytt schema annotations]:
  https://carvel.dev/ytt/docs/latest/lang-ref-ytt-schema/
