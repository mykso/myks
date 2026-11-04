---
title: Helm
sidebar:
  order: 4
---

The Helm step renders every Helm chart an application has downloaded with
`helm template`, using values files assembled along the inheritance tree. Use it
to deploy any upstream chart: set per-environment values without copying the
chart, and patch what the chart cannot configure with
[ytt overlays](../../guides/ytt/) afterwards.

`helm` is called as an external binary and must be on your `PATH`. The
[container image](../../guides/installation/) includes it.

## Getting a chart

myks renders every directory under the application's `vendor/charts/` that
contains a `Chart.yaml`; other entries are skipped with a warning. Charts get
there through a vendir source whose `path` starts with `charts/`, see
[sources.md](../../guides/sources/):

```yaml
#! prototypes/webapp/vendir/base.ytt.yaml
apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
  - path: charts/podinfo
    contents:
      - path: .
        lazy: true
        helmChart:
          name: podinfo
          version: 6.17.0
          repository:
            url: https://stefanprodan.github.io/podinfo
```

:::caution[Important]

myks identifies a chart by its **directory name** under `vendor/charts/`, not by
the `name` in its `Chart.yaml`. Values files and per-chart settings use that
directory name. This lets one application render the same chart twice, for
example as `charts/redis-cache` and `charts/redis-queue`.

:::

Charts are rendered in alphabetical order of their directory names and their
output is concatenated.

## Values files

Values files live in `helm/` directories and are named after the chart
directory: `podinfo.yaml`, or any `podinfo.<anything>yaml` such as
`podinfo.ytt.yaml` or `podinfo.resources.yaml`. Files named `_global.*yaml`
apply to every chart of the application.

For the `frontend` application, based on the `webapp` prototype, in the
`envs/prod/eu` environment, myks collects values files from these directories,
in this order:

1. `prototypes/webapp/helm/`
2. `envs/_proto/webapp/helm/`
3. `envs/prod/_proto/webapp/helm/`
4. `envs/prod/eu/_proto/webapp/helm/`
5. `envs/_apps/frontend/helm/`
6. `envs/prod/_apps/frontend/helm/`
7. `envs/prod/eu/_apps/frontend/helm/`

```
.
├── prototypes/webapp/helm/podinfo.yaml       # 1: defaults for every application of the prototype
└── envs/
    ├── _proto/webapp/helm/podinfo.yaml       # 2: every webapp application, everywhere
    ├── _apps/frontend/helm/podinfo.yaml      # 5: frontend, everywhere
    └── prod/
        ├── _apps/frontend/helm/podinfo.yaml  # 6: frontend in every prod environment
        └── eu/
            └── _apps/frontend/helm/podinfo.yaml  # 7: frontend in prod-eu only
```

All `_global.*yaml` files go first, in the order above, then all `podinfo.*yaml`
files, in the order above. Within one directory, files apply in alphabetical
order. Later files win: maps merge key by key, lists and scalars are replaced,
the way `helm template -f a.yaml -f b.yaml` merges them.

`_env/helm/` directories are not read: Helm values are always specific to a
chart, so they belong to a prototype or an application. Use
`_proto/<prototype>/helm/` to change all applications of a prototype at one
level of the tree.

:::tip

`myks inspect apps prod/eu frontend` lists the values files of each chart in
merge order, and `--provenance` shows which file sets each values key. See
[Inspecting and debugging](../../guides/inspect/).

:::

### Values files are ytt templates

Each values file is rendered by ytt together with the application's
[data values](../../guides/data-values/) before merging, so values can be
computed:

```yaml
#! prototypes/webapp/helm/podinfo.yaml
#@ load("@ytt:data", "data")
---
ui:
  message: #@ "Hello from " + data.values.environment.id
replicaCount: #@ 2 if data.values.environment.id.startswith("prod") else 1
```

The chart being rendered is available as `data.values.myks.context.helm.chart`,
which lets one `_global` file adapt to each chart. A values file can hold
several YAML documents; they merge in order. The merged result is written to
`.myks/<env-dir>/_apps/<app>/helm/<chart>.yaml` and passed to `helm template`.

## Configuration

The `helm` scope of the data values configures the step. Set it in any
`app-data*.yaml` or `env-data*.yaml` file, at any level of the tree.

| Key                 | Default          | Effect                                                                                                                 |
| ------------------- | ---------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `namespace`         | application name | Passed as `--namespace`.                                                                                               |
| `includeCRDs`       | `true`           | Adds `--include-crds`.                                                                                                 |
| `kubeVersion`       | `''`             | Passed as `--kube-version` when set. Set it per environment to match each cluster.                                     |
| `capabilities`      | `[]`             | Each entry is passed as `--api-versions`, for example `monitoring.coreos.com/v1`, so charts render optional resources. |
| `buildDependencies` | `false`          | Run `helm dependencies build` during sync, see [below](#chart-dependencies).                                           |
| `charts`            | `[]`             | Per-chart overrides, see below.                                                                                        |

Every entry of `helm.charts` overrides the settings for one chart:

| Key                 | Default                  | Effect                                                       |
| ------------------- | ------------------------ | ------------------------------------------------------------ |
| `name`              | required                 | Chart directory name under `vendor/charts/`. Must be unique. |
| `releaseName`       | chart directory name     | Release name passed to `helm template`.                      |
| `namespace`         | `helm.namespace`         | `--namespace` for this chart.                                |
| `includeCRDs`       | `helm.includeCRDs`       | `--include-crds` for this chart.                             |
| `buildDependencies` | `helm.buildDependencies` | Dependency build for this chart.                             |

```yaml
#! prototypes/webapp/app-data.ytt.yaml
#@data/values
---
helm:
  kubeVersion: '1.33.0'
  charts:
    - name: podinfo
      releaseName: frontend
```

`charts` is a ytt list, so a lower level that writes `charts:` again appends
entries instead of changing them. Match the existing entry by name to change it:

```yaml
#! envs/prod/eu/_apps/frontend/app-data.ytt.yaml
#@ load("@ytt:overlay", "overlay")
#@data/values
---
helm:
  charts:
    #@overlay/match by="name"
    - name: podinfo
      namespace: frontend-eu
```

myks always passes `--skip-tests`, so chart test hooks are not rendered. It
warns about `helm.charts` entries and values files that match no downloaded
chart, which usually means a typo or a renamed source.

The exact `helm template` command of every chart is logged with
`--log-level debug`.

## Chart dependencies

Charts downloaded from a chart repository usually bundle their dependencies.
Charts taken from a Git repository often do not. Set `buildDependencies: true`
(globally or for one chart) and myks runs, during sync:

1. `helm repo add` for every `http(s)` repository listed in the chart's
   `Chart.yaml` dependencies, in a repository cache private to the application
   (`.myks/<env-dir>/_apps/<app>/helm-cache/`), so your own Helm configuration
   stays untouched;
2. `helm dependencies build --skip-refresh` for the chart.

A chart shared by several applications through the
[cache](../../guides/optimizations/#centralized-cache) is built once per run.
