# Concepts

Myks is two things: a set of directory conventions for a GitOps repository, and
a CLI that turns such a repository into plain Kubernetes manifests. The CLI
orchestrates a few well-known tools: [vendir] downloads sources, [Helm] renders
charts, [ytt] computes values and patches YAML, and [kbld] pins images. Myks
decides which files each tool sees, in which order, and for which cluster.

This page explains the model. The [getting started](getting-started.md) tutorial
shows it in action.

## Core entities

**Prototype**  
A reusable definition of an application: where its sources come from (Helm
charts, Git repositories, plain YAML) and its default values. The simplest
prototype is an empty directory. Prototypes live in `prototypes/<name>/`.

**Application**  
An instance of a prototype in an environment. It can override the prototype's
versions and values, add or remove sources, and patch the rendered manifests.
One environment can run the same prototype several times under different names.
An application is deployed as one unit (one Argo CD `Application`).

**Environment**  
A Kubernetes cluster, and the applications that run on it. Environments are
directories under `envs/` that form an inheritance tree: every environment
inherits configuration from the directories above it.

Only environments and applications appear in the output: `rendered/envs/` holds
one directory per environment, with one directory per application inside.

## Project layout

```text
.
├── .myks.yaml       # myks settings (configuration.md)
├── envs/            # the environment tree
├── prototypes/      # prototypes, one directory each
│   └── _vendir/     # optional: vendir files shared by every prototype
├── lib/             # optional: ytt libraries available to every ytt call
├── plugins/         # optional: plugin executables (plugins.md)
├── rendered/        # output, committed to Git
│   ├── envs/<environment-id>/<application>/
│   └── argocd/<environment-id>/
└── .myks/           # cache and intermediate files, not committed
```

Directory names (`envs`, `prototypes`, `_apps`, …) can be changed with
[`naming-conventions`](configuration.md#naming-conventions).

### Prototypes and applications

A prototype and an application share the same structure. Every part is optional:

```text
<name>/
├── app-data*.yaml   # data values: defaults and schema (data-values.md)
├── vendir/          # source definitions for the sync stage (sources.md)
├── helm/            # Helm values files, one per chart (helm.md)
├── ytt/             # ytt templates and overlays (ytt.md)
├── static/          # files copied to the output unchanged (static-files.md)
├── argocd/          # overlays for the generated Argo CD Application (argocd.md)
└── lib/             # ytt libraries (ytt-libraries.md)
```

A prototype lives in `prototypes/<name>/`. Application-specific files live in
the environment tree, in `_apps/<application>/` directories, at any level.

### Environments

```text
<environment>/
├── env-data*.yaml   # data values of this level, including environment.id
├── _apps/<app>/     # files for one application, in this subtree
├── _proto/<proto>/  # files for every application of a prototype, in this subtree
├── _env/            # files for every application of this subtree
│   ├── ytt/         #   overlays applied to every application (ytt.md#where-myks-looks-for-files)
│   ├── static/      #   static files added to every application
│   └── argocd/      #   overlays for the generated Argo CD resources
└── <child>/         # nested environments or grouping directories
```

A directory becomes a rendered environment when one of its `env-data*.yaml`
files sets `environment.id`. The id names the output directory
(`rendered/envs/<id>/`), so you can name and nest environment directories freely
and reorganize them later without changing the output.

Myks reads the id before running ytt, so it must be a plain value in the first
YAML document of an `env-data` file in the environment's own directory: not a
ytt expression, and not inherited from a parent level.

A directory without `environment.id` is a grouping level: it renders nothing,
and only carries configuration that the environments below it inherit. In this
tree, only the three leaves are environments:

```text
envs/
├── env-data.ytt.yaml          # defaults for everything
├── prod/
│   ├── env-data.ytt.yaml      # production defaults
│   ├── eu/env-data.ytt.yaml   # environment.id: prod-eu
│   └── us/env-data.ytt.yaml   # environment.id: prod-us
└── staging/
    └── eu/env-data.ytt.yaml   # environment.id: staging-eu
```

The commands select environments by directory, so `myks render prod` processes
both production clusters, and `myks render prod/eu` only one.

### Declaring applications

An environment declares its applications in `environment.applications`:

```yaml
#@data/values
---
environment:
  id: prod-eu
  applications:
    - proto: webapp # name defaults to the prototype name
    - proto: webapp # a second instance of the same prototype
      name: webapp-canary
```

Lists of applications are appended down the tree. A grouping level can declare
the applications every cluster below it runs, and each environment adds its own.

## Inheritance

Inheritance is what keeps a large repository small. For an application, myks
collects files from the prototype and from every level of the environment's
path, root first, and later files win.

For an application `frontend` of prototype `webapp` in the environment
`envs/prod/eu/`, Helm values are collected in this order:

```text
prototypes/webapp/helm/podinfo.yaml              # 1  prototype defaults
envs/_proto/webapp/helm/podinfo.yaml             # 2  every webapp, everywhere
envs/prod/_proto/webapp/helm/podinfo.yaml        # 3  every webapp in prod
envs/prod/eu/_proto/webapp/helm/podinfo.yaml     # 4  every webapp in prod/eu
envs/_apps/frontend/helm/podinfo.yaml            # 5  frontend, everywhere
envs/prod/_apps/frontend/helm/podinfo.yaml       # 6  frontend in prod
envs/prod/eu/_apps/frontend/helm/podinfo.yaml    # 7  frontend in prod/eu
```

The same pattern holds for every kind of file, with small differences per step:

| Files               | Prototype | `_proto/<proto>/` | `_env/` | `_apps/<app>/` | Details                           |
| ------------------- | :-------: | :---------------: | :-----: | :------------: | --------------------------------- |
| Data values         |     ●     |         ●         |    —    |       ●        | [Data values](data-values.md)     |
| Sources (`vendir/`) |     ●     |         —         |    —    |       ●        | [Sources](sources.md)             |
| Helm values         |     ●     |         ●         |    —    |       ●        | [Helm](helm.md)                   |
| ytt overlays        |     ●     |         ●         |    ●    |       ●        | [ytt](ytt.md)                     |
| Static files        |     ●     |         ●         |    ●    |       ●        | [Static files](static-files.md)   |
| Argo CD overlays    |     ●     |         —         |    ●    |       ●        | [Argo CD](argocd.md)              |
| ytt libraries       |     ●     |         ●         |    —    |       ●        | [ytt libraries](ytt-libraries.md) |

`_env/ytt/` overlays are applied in a separate step, after the application's own
ytt step (see [render pipeline](#render-pipeline)).

`myks inspect apps <env> <app>` lists the exact files used for one application,
and `--provenance` shows which file sets each final value. See
[Inspecting and debugging](inspect.md).

## Processing stages

Every command starts with **init**: myks finds the environments and applications
to process, and computes their data values. Then `myks render` runs two stages
per application, in parallel across applications:

1. **Sync** downloads the application's sources with vendir into
   `.myks/envs/<path>/_apps/<app>/vendor/`, through a cache shared by all
   applications (`.myks/vendir-cache/`). Helm chart dependencies are built here
   when enabled. See [Sources](sources.md).
2. **Render** runs the render pipeline below and writes the result to
   `rendered/`.

`myks render --sync` and `myks render --render` run one stage only. Rendering
without syncing is useful while iterating on values and overlays, once the
sources are downloaded.

### Render pipeline

The render pipeline is a fixed sequence of steps. **Additive** steps produce new
YAML that is added to the output so far. **Mutative** steps take the output so
far as input and transform it.

| Step                                            | Kind     | What it does                                                                                           |
| ----------------------------------------------- | -------- | ------------------------------------------------------------------------------------------------------ |
| [helm](helm.md)                                 | additive | `helm template` for every chart in `vendor/charts/`                                                    |
| [ytt-pkg](ytt.md#ytt-packages)                  | additive | renders vendored ytt packages from `vendor/ytt-pkg/`                                                   |
| [ytt](ytt.md)                                   | mutative | renders the application's ytt templates and overlays, together with the output so far                  |
| [global-ytt](ytt.md#where-myks-looks-for-files) | mutative | applies `_env/ytt/` overlays of the environment tree                                                   |
| [kbld](kbld.md)                                 | mutative | resolves image tags to digests and rewrites image references (off by default)                          |
| slice                                           | —        | sorts keys and writes one file per resource to `rendered/envs/<env-id>/<app>/`                         |
| [static](static-files.md)                       | —        | copies static files to `rendered/envs/<env-id>/<app>/static/`                                          |
| [argocd](argocd.md)                             | —        | writes the Argo CD `Application` to `rendered/argocd/<env-id>/` (and once per environment its project) |

The ytt step can produce resources on its own, so an application without any
Helm chart (plain YAML from Git, or templates written in the repository) works
the same way.

### Rendered output

The slice step writes each resource to `<kind>-<name>.yaml`, lowercase, for
example `deployment-httpbingo.yaml`. When two resources of the same kind and
name exist in different namespaces, set `render.includeNamespace: true` to name
files `<kind>-<name>_<namespace>.yaml`.

The step replaces the application's output directory on every render, so a
resource removed from the sources disappears from `rendered/` too. Keys are
sorted, so the output only changes when the manifests change.

After rendering, myks deletes the output of applications that are no longer
listed in a processed environment. `myks render ALL` also deletes the output of
environments that no longer exist; Smart Mode runs and `myks render ALL ALL` do
not. `myks cleanup --manifests` does both explicitly.

### The `.myks/` directory

`.myks/` holds the vendir cache, downloaded sources, and the intermediate files
of every step: rendered vendir configurations, merged Helm values, the data
values schema (`.myks/tmp/data-schema.ytt.yaml`). It is safe to delete, at the
cost of downloading everything again. `myks init` adds it to `.gitignore`. To
speed up CI, cache it between runs (see [Optimizations](optimizations.md)).

## Data values

Each application has one set of configuration values, its **data values**,
computed from `env-data*.yaml` files at every level of the environment tree and
from `app-data*.yaml` files of the prototype, `_proto/` and `_apps/`
directories. Every step reads them: vendir configurations, Helm values files,
ytt templates and the Argo CD templates.

Some top-level keys configure myks itself (`environment`, `argocd`, `helm`,
`kbld`, `render`, `yttPkg`); `application` is free for your own values. See
[Data values](data-values.md).

> [!NOTE]  
> Data values are written as ytt data values files today. A new configuration
> layer in [KCL](https://www.kcl-lang.io/) is available as a preview: it
> replaces the data values files with typed KCL code and keeps everything else
> on this page unchanged. See [KCL mode](kcl-mode.md).

## What myks does not do

- It does not connect to clusters. Output is files; deploy them with Argo CD,
  Flux, kapp, `kubectl`, or a [plugin](plugins.md).
- It does not build container images.
- It does not decrypt secrets. Keep them encrypted in the output (for example
  sops files as [static files](static-files.md)) and decrypt them at deploy
  time, or use an operator such as External Secrets.

[Helm]: https://helm.sh/
[kbld]: https://carvel.dev/kbld/
[vendir]: https://carvel.dev/vendir/
[ytt]: https://carvel.dev/ytt/
