# ArgoCD Integration

Myks generates ArgoCD resources for every environment and application it
renders. The `argocd` scope of the data values exposes the commonly used fields
(see `argocd:` in
[data-schema.ytt.yaml](/internal/myks/assets/data-schema.ytt.yaml)). Any other
field of the ArgoCD `Application` or `AppProject` can be set with ytt overlays,
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

## KCL mode

The same overlay directories and rules apply when the configuration layer is
written in KCL (see [migration](/docs/migration.md)): the `argocd` values from
KCL feed the same templates, and `argocd/` overlays are collected the same way.
