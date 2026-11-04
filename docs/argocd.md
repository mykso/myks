# Argo CD Integration

Myks generates Argo CD resources for every environment and application it
renders. The `argocd` scope of the data values exposes the commonly used fields
(see `argocd:` in
[data-schema.ytt.yaml](/internal/myks/assets/data-schema.ytt.yaml)). Any other
field of the Argo CD `Application` or `AppProject` can be set with ytt overlays,
as described below.

## Generated resources

With `argocd.enabled: true` (the default), myks writes the following files to
`rendered/argocd/<environment.id>/`:

| File                     | Content                                                                                                                    |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------- |
| `env-<environment>.yaml` | `AppProject` (unless `argocd.project.enabled: false`) and the cluster `Secret` (unless `argocd.env.generateSecret: false`) |
| `app-<application>.yaml` | `Application`                                                                                                              |

The cluster `Secret` contains placeholder values
(`ARGOCD_CLUSTER_CONNECT_CONFIG`, `ARGOCD_CLUSTER_SERVER_URL`), which are meant
to be replaced with an overlay. `myks init` creates an example of such an
overlay in `envs/_env/argocd/secret.overlay.ytt.yaml`.

## Configuration

Set these keys in any data-values file. The `Application` of an application is
rendered from that application's data values; the `AppProject` and `Secret` of
an environment are rendered from the environment's `env-data*.yaml` files only.
Set `argocd.namespace`, `argocd.env.*` and `argocd.project.*` in
`env-data*.yaml`, so that both renders see the same values.

In the table, `<env>` stands for `argocd.env.name`, or `environment.id` when
that is empty.

| Key                                    | Default                                              | Effect                                                                                                                                                            |
| -------------------------------------- | ---------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `argocd.enabled`                       | `true`                                               | In an application's data values: render its `Application`. In an environment's data values: render the `AppProject` and `Secret`.                                 |
| `argocd.namespace`                     | `argocd`                                             | `metadata.namespace` of every generated resource.                                                                                                                 |
| `argocd.app.name`                      | application name                                     | Last part of the `Application` name, which is `<app.prefix><env>-<app.name>`.                                                                                     |
| `argocd.app.prefix`                    | `''`                                                 | Prefix of the `Application` name.                                                                                                                                 |
| `argocd.app.finalizers`                | `['resources-finalizer.argocd.argoproj.io']`         | `metadata.finalizers`. A list: use `#@overlay/replace` to change it.                                                                                              |
| `argocd.app.destination.name`          | `<env.prefix><env>`                                  | `spec.destination.name`.                                                                                                                                          |
| `argocd.app.destination.server`        | `''`                                                 | `spec.destination.server`; when set, used instead of `destination.name`.                                                                                          |
| `argocd.app.destination.namespace`     | `argocd.app.name`                                    | `spec.destination.namespace`.                                                                                                                                     |
| `argocd.app.source.path`               | `rendered/envs/<environment.id>/<application>`       | `spec.source.path`, prefixed with the project's path inside the Git repository when the project is in a subdirectory.                                             |
| `argocd.app.source.repoURL`            | URL of the `origin` Git remote                       | `spec.source.repoURL`.                                                                                                                                            |
| `argocd.app.source.targetRevision`     | current Git branch                                   | `spec.source.targetRevision`.                                                                                                                                     |
| `argocd.app.source.plugin`             | none                                                 | `spec.source.plugin`, for a config management plugin such as argocd-vault-plugin.                                                                                 |
| `argocd.app.syncPolicy.prune`          | `true`                                               | `spec.syncPolicy.automated.prune`.                                                                                                                                |
| `argocd.app.syncPolicy.selfHeal`       | `true`                                               | `spec.syncPolicy.automated.selfHeal`.                                                                                                                             |
| `argocd.app.syncPolicy.syncOptions`    | `{CreateNamespace: 'true', ServerSideApply: 'true'}` | Map of [sync options](https://argo-cd.readthedocs.io/en/stable/user-guide/sync-options/), rendered as sorted `Key=value` entries. A `null` value drops an option. |
| `argocd.env.name`                      | `environment.id`                                     | Environment name used in generated names.                                                                                                                         |
| `argocd.env.prefix`                    | `''`                                                 | Prefix of the cluster name: the `Secret` name and the default destination names.                                                                                  |
| `argocd.env.generateSecret`            | `true`                                               | Render the cluster `Secret` with placeholder values.                                                                                                              |
| `argocd.project.enabled`               | `true`                                               | Render the `AppProject`. Set `false` when another tool manages it, and set `project.name` to its name.                                                            |
| `argocd.project.name`                  | `<project.prefix><env>`                              | `AppProject` name and `spec.project` of every `Application` of the environment.                                                                                   |
| `argocd.project.prefix`                | `''`                                                 | Prefix of the `AppProject` name.                                                                                                                                  |
| `argocd.project.destination.name`      | `<env.prefix><env>`                                  | `spec.destinations[0].name` of the `AppProject`.                                                                                                                  |
| `argocd.project.destination.server`    | `''`                                                 | `spec.destinations[0].server`; when set, used instead of `name`.                                                                                                  |
| `argocd.project.destination.namespace` | `'*'`                                                | `spec.destinations[0].namespace`.                                                                                                                                 |

Each sync option accepts only the values Argo CD documents for it, for example
`Prune: confirm` or `PrunePropagationPolicy: foreground`; the schema rejects
others.

> [!TIP]  
> In CI, a checkout is often a detached `HEAD`, which makes the current branch
> `HEAD`. Set `argocd.app.source.targetRevision` (and `repoURL`, if the CI
> remote differs) explicitly in the root `env-data` file.

## Overlays

ytt overlay files are picked up from `argocd/` directories in three locations:

| Location                              | Applied to                                                           |
| ------------------------------------- | -------------------------------------------------------------------- |
| `prototypes/<prototype>/argocd/`      | `Application` render                                                 |
| `envs/**/_env/argocd/`                | `Application` render and environment render (`AppProject`, `Secret`) |
| `envs/**/_apps/<application>/argocd/` | `Application` render                                                 |

`_env/argocd/` and `_apps/<application>/argocd/` directories are collected at
every level of the environment hierarchy, from the root down to the environment
itself. The `Application` render applies the prototype overlays first, then the
`_env/argocd/` overlays, then the `_apps/<application>/argocd/` overlays.

### Overlays in `_env/argocd/` must tolerate missing documents

`_env/argocd/` overlays run for both renders, but each render produces only some
of the resources: the `Application` render has no `AppProject` or `Secret`, and
the environment render has no `Application`. An overlay that matches a single
kind there needs `expects="0+"`. Without it, the render that lacks the matched
kind fails with:

```text
Expected number of matched nodes to be 1, but was 0
```

## Examples

### Application

`envs/dev/_apps/httpbingo/argocd/spec.overlay.ytt.yaml`:

```yaml
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.subset({"kind": "Application"})
---
#@overlay/match-child-defaults missing_ok=True
spec:
  revisionHistoryLimit: 3
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jsonPointers: ['/spec/replicas']
  syncPolicy:
    retry:
      limit: 5
      backoff: { duration: 5s, factor: 2, maxDuration: 3m }
```

### AppProject

`envs/_env/argocd/project.overlay.ytt.yaml`:

```yaml
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.subset({"kind": "AppProject"}), expects="0+"
---
spec:
  #@overlay/match missing_ok=True
  sourceNamespaces: ['argocd-apps']
  #@overlay/replace
  sourceRepos:
    - https://github.com/example/gitops.git
```

`#@overlay/replace` swaps the generated `sourceRepos: ["*"]` for the given list;
without it, ytt would append to the list.

### Manual sync

The generated `Application` always has `spec.syncPolicy.automated`. Remove it to
sync an application by hand:

`envs/prod/_apps/frontend/argocd/manual-sync.overlay.ytt.yaml`:

```yaml
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.subset({"kind": "Application"})
---
spec:
  syncPolicy:
    #@overlay/remove
    automated:
```

## KCL mode

The same overlay directories and rules apply when the configuration layer is
written in KCL (see [migration](/docs/migration.md)): the `argocd` values from
KCL feed the same templates, and `argocd/` overlays are collected the same way.
