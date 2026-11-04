# Sources

The sync stage downloads what an application is built from: Helm charts, Git
repositories, GitHub release assets, single files. Myks hands the work to
[vendir], adding three things on top: the vendir configuration is a ytt template
with access to the application's [data values](data-values.md), downloads are
cached and shared between applications, and applications sync in parallel.

## Where the configuration comes from

For each application, myks renders one vendir configuration from the files in:

1. `prototypes/_vendir/`, shared by every prototype (optional);
2. `prototypes/<proto>/vendir/`;
3. `_apps/<app>/vendir/` at every level of the environment tree.

The files are ytt templates, overlays and data values, rendered together with
the application's data values. An application without any `vendir/` directory of
its own has nothing to sync, and `prototypes/_vendir/` is not used for it.

The rendered configuration is written to
`.myks/envs/<env-path>/_apps/<app>/vendir.yaml`, and myks rejects it if vendir
would. Sources land in the application's `vendor/` directory, where each render
step looks for its input:

| Path in `vendor/` | Used by                                              |
| ----------------- | ---------------------------------------------------- |
| `charts/<name>/`  | [helm](helm.md): one chart per directory             |
| `ytt/`            | [ytt](ytt.md): files added before your own templates |
| `ytt-pkg/<name>/` | [ytt-pkg](ytt.md#ytt-packages)                       |

No step reads other paths of `vendor/`, so the `path` of a vendir directory
decides which step uses it.

See the [vendir.yml reference] for every source type and option.

## Examples

### Helm chart from a repository

```yaml
#! prototypes/cert-manager/vendir/vendir.yaml
apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
  - path: charts/cert-manager
    contents:
      - path: .
        lazy: true
        helmChart:
          name: cert-manager
          version: v1.16.2
          repository:
            url: https://charts.jetstack.io
```

### Helm chart from an OCI registry

```yaml
directories:
  - path: charts/podinfo
    contents:
      - path: .
        lazy: true
        helmChart:
          name: podinfo
          version: 6.7.1
          repository:
            url: oci://ghcr.io/stefanprodan/charts
```

### Helm chart from a Git repository

Charts that are not published, or not published yet, can be taken from a
directory of a Git repository:

```yaml
directories:
  - path: charts/istio-ingress
    contents:
      - path: .
        lazy: true
        git:
          url: https://github.com/istio/istio
          ref: 1.24.2
          depth: 1
        includePaths:
          - manifests/charts/gateways/istio-ingress/**/*
        newRootPath: manifests/charts/gateways/istio-ingress
```

Charts in Git often do not bundle their dependencies. Enable
`helm.buildDependencies` so that myks runs `helm dependency build` after the
sync (see [Helm](helm.md)).

### Plain manifests from Git

Files synced to `ytt/` are rendered by the ytt step, which turns any plain YAML
into output. This installs Argo CD from its upstream manifests:

```yaml
directories:
  - path: ytt/argocd
    contents:
      - path: .
        lazy: true
        git:
          url: https://github.com/argoproj/argo-cd
          ref: v2.13.3
          depth: 1
        includePaths:
          - manifests/install.yaml
        newRootPath: manifests
```

### GitHub release asset

```yaml
directories:
  - path: ytt/prometheus-operator
    contents:
      - path: .
        lazy: true
        githubRelease:
          slug: prometheus-operator/prometheus-operator
          tag: v0.79.2
          disableAutoChecksumValidation: true
          assetNames: ['bundle.yaml']
```

## Templating source definitions

Writing the full vendir configuration in every prototype is verbose. The
prototypes that `myks init` scaffolds split it into a template and a values
file, so that the version is a data value that environments can override:

```yaml
#! prototypes/httpbingo/vendir/base.ytt.yaml
#@ load("@ytt:data", "data")
#@ app = data.values.application
---
apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
  - path: #@ "charts/" + app.name
    contents:
      - path: .
        lazy: true
        helmChart:
          name: #@ app.name
          version: #@ app.version
          repository:
            url: #@ app.url
```

```yaml
#! prototypes/httpbingo/vendir/vendir-data.ytt.yaml
#@data/values-schema
---
#@overlay/match-child-defaults missing_ok=True
application:
  name: httpbingo
  url: https://estahn.github.io/charts
  version: 0.1.1
```

The same idea scales to several charts per prototype with a loop:

```yaml
#! prototypes/<proto>/vendir/base.ytt.yaml
#@ load("@ytt:data", "data")
---
apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
  #@ for/end chart in data.values.application.charts:
  - path: #@ "charts/" + chart.name
    contents:
      - path: .
        lazy: true
        helmChart:
          name: #@ chart.name
          version: #@ chart.version
          repository:
            url: #@ chart.url
```

A template like this is written once. Put it in `prototypes/_vendir/` and every
prototype only declares its `application.charts` values.

## Overriding a source for one environment

Because the files under `_apps/<app>/vendir/` are rendered together with the
prototype's, an environment can change a source in two ways.

Override the data value, if the prototype templates its sources:

```yaml
#! envs/prod/_apps/httpbingo/vendir/vendir-data.ytt.yaml
#@data/values
---
application:
  version: 0.1.0
```

Or overlay the rendered vendir configuration, which works for any prototype.
This adds a chart to one application:

```yaml
#! envs/prod/_apps/frontend/vendir/redis.ytt.yaml
#@ load("@ytt:overlay", "overlay")
#@overlay/match by=overlay.subset({"kind": "Config"})
---
directories:
  - path: charts/redis
    contents:
      - path: .
        lazy: true
        helmChart:
          name: redis
          version: 20.6.3
          repository:
            url: oci://registry-1.docker.io/bitnamicharts
```

### Testing an unreleased chart from a branch

To try a chart change before it is released, point the application at the branch
that contains it, and disable `lazy` so that new commits on the branch are
fetched:

```yaml
#! envs/staging/_apps/frontend/vendir/preview.ytt.yaml
#@ load("@ytt:overlay", "overlay")
#@overlay/match by=overlay.subset({"kind": "Config"})
---
directories:
  #@overlay/match by="path"
  - path: charts/podinfo
    contents:
      #@overlay/match by="path"
      #@overlay/replace
      - path: .
        lazy: false
        git:
          url: https://github.com/stefanprodan/podinfo
          ref: origin/my-feature
          depth: 1
        includePaths:
          - charts/podinfo/**/*
        newRootPath: charts/podinfo
```

A branch name needs the `origin/` prefix. Delete the file to go back to the
released chart.

## Lazy syncing

`lazy: true` tells vendir to skip a source when its configuration has not
changed since the last sync. Most syncs then download nothing. Use it for every
source with a fixed version: a chart version, a tag, a commit.

Do not use it for moving references such as branches. With `lazy: true`, vendir
does not notice that the branch moved.

Lazy sources also skip the cache: when a lazy source is already in
`.myks/vendir-cache/`, myks does not run vendir for it at all.

## Cache

Myks keeps downloaded sources in `.myks/vendir-cache/`, keyed by a hash of the
source definition, and links them into each application's `vendor/` directory.
Ten applications using the same chart version download it once.

```shell
myks cleanup --cache --dry-run   # list cache entries no application uses
myks cleanup --cache             # delete them
```

To speed up CI, persist `.myks/vendir-cache/` between runs (see
[Optimizations](optimizations.md#speed-up-cicd-pipelines)).

## Private repositories and registries

vendir authenticates with Kubernetes-style `Secret` resources referenced by
`secretRef`. Myks generates these secrets from environment variables, so no
credentials are stored in the repository:

```yaml
directories:
  - path: charts/internal-app
    contents:
      - path: .
        helmChart:
          name: internal-app
          version: 1.2.3
          repository:
            url: https://charts.example.com
            secretRef:
              name: mycreds
```

```shell
export VENDIR_SECRET_MYCREDS_USERNAME=robot
export VENDIR_SECRET_MYCREDS_PASSWORD="$CHART_REPO_TOKEN"
myks render
```

The secret name is the part between `VENDIR_SECRET_` and `_USERNAME` or
`_PASSWORD`, lowercased. Only username and password secrets are generated.

## Keeping versions up to date with Renovate

Versions in data values files are not in a format Renovate knows, but a regex
manager can find them. With a marker comment above the source keys, in this
order:

```yaml
application:
  #! renovate: datasource=helm
  name: httpbingo
  url: https://estahn.github.io/charts
  version: 0.1.1
```

this `renovate.json5` entry updates `version`:

```json5
{
  customManagers: [
    {
      customType: 'regex',
      managerFilePatterns: ['/(^|/)vendir/.+\\.ya?ml$/'],
      matchStrings: [
        '#! renovate: datasource=(?<datasource>helm)\\s*\\n\\s*name: (?<depName>\\S+)\\s*\\n\\s*url: (?<registryUrl>\\S+)\\s*\\n\\s*version: (?<currentValue>\\S+)',
      ],
    },
  ],
}
```

The keys must stay in this order, with nothing between them, or the pattern
stops matching.

## Debugging a sync

`myks render --sync --log-level debug` prints every vendir command myks runs.
Myks syncs each source into its own cache entry, so the commands differ from a
plain `vendir sync` of the rendered `vendir.yaml`. `myks vendir` runs the
embedded vendir with the usual vendir arguments, for trying a configuration by
hand.

[vendir]: https://carvel.dev/vendir/
[vendir.yml reference]: https://carvel.dev/vendir/docs/latest/vendir-spec/
