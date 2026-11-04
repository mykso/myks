# Getting started

This tutorial builds a small GitOps repository for two clusters. On the way you
render a Helm chart, add a cluster, override a value for one cluster only, patch
the chart output without touching the chart, and pin a chart version per
cluster. It takes about ten minutes.

You need [myks](installation.md), [Helm](https://helm.sh/docs/intro/install/)
and Git.

## 1. Create a project

Myks uses Git to find what changed, so start with a repository that has at least
one commit:

```shell
mkdir my-gitops && cd my-gitops
git init
git commit --allow-empty -m 'Initial commit'
myks init
```

`myks init` writes a sample project:

```text
.
├── .myks.yaml                      # myks settings
├── .gitignore                      # ignores the .myks/ cache directory
├── envs/                           # the environment tree
│   ├── env-data.ytt.yaml           # values every environment inherits
│   ├── _env/
│   │   ├── argocd/                 # overlays for the generated Argo CD resources
│   │   └── ytt/common.ytt.yaml     # overlay applied to every application
│   └── mykso/
│       └── dev/                    # an environment: one cluster
│           ├── env-data.ytt.yaml   # its id and its applications
│           └── _apps/
│               └── httpbingo/
│                   └── helm/httpbingo.yaml   # Helm values for this cluster only
└── prototypes/                     # reusable application definitions
    ├── argocd/
    └── httpbingo/
        ├── vendir/                 # where to download the chart from
        └── helm/httpbingo.yaml     # default Helm values
```

Three ideas are enough to read it:

- A **prototype** (`prototypes/httpbingo/`) says where an application's sources
  come from and holds its defaults.
- An **environment** (`envs/mykso/dev/`) is one cluster. Its `env-data` file
  sets its id and lists the applications that run on it.
- **Inheritance** follows directories: `envs/mykso/dev/` inherits everything
  from `envs/mykso/` and `envs/`.

Look at the environment definition:

```yaml
#! envs/mykso/dev/env-data.ytt.yaml
#@data/values
---
environment:
  id: mykso-dev
  applications:
    - proto: argocd
    - proto: httpbingo
```

And at the prototype's source definition, a [vendir] configuration rendered with
[ytt]:

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
        helmChart:
          name: #@ app.name
          version: #@ app.version
          repository:
            url: #@ app.url
        lazy: true
```

The chart name, version and URL come from `vendir-data.ytt.yaml` next to it.

## 2. Render

```shell
myks render
```

Myks downloads the charts (the **sync** stage), then renders them (the
**render** stage). The result is plain YAML, one file per resource:

```text
rendered/
├── argocd/mykso-dev/
│   ├── app-argocd.yaml             # Argo CD Application per application
│   ├── app-httpbingo.yaml
│   └── env-mykso-dev.yaml          # AppProject and cluster Secret
└── envs/mykso-dev/
    ├── argocd/                     # ~70 files of the Argo CD installation
    └── httpbingo/
        ├── deployment-httpbingo.yaml
        ├── service-httpbingo.yaml
        └── serviceaccount-httpbingo.yaml
```

Commit everything. From now on, the diff of `rendered/` shows the effect of
every change.

```shell
git add -A && git commit -m 'Render dev'
```

## 3. Add a second cluster

Create an environment for production that runs only `httpbingo`:

```shell
mkdir -p envs/mykso/prod
cat > envs/mykso/prod/env-data.ytt.yaml <<'EOF'
#@data/values
---
environment:
  id: mykso-prod
  applications:
    - proto: httpbingo
EOF
```

Before rendering, ask [Smart Mode](smart-mode.md) what it would process:

```console
$ myks render --smart-mode.only-print
Smart Mode detected:
→ envs/mykso/prod
    ALL
```

Myks compared the working tree with `HEAD` and selected only the new
environment. Render and commit:

```shell
myks render
git add -A && git commit -m 'Add prod'
```

`rendered/envs/mykso-prod/httpbingo/` now holds the same manifests as dev: prod
inherited the prototype's defaults and the root values from `envs/`.

## 4. Override a value for one cluster

Production needs three replicas. Helm values are looked up in the prototype
first, then in the environment tree, and later files win:

```shell
mkdir -p envs/mykso/prod/_apps/httpbingo/helm
echo 'replicaCount: 3' > envs/mykso/prod/_apps/httpbingo/helm/httpbingo.yaml
myks render
git diff rendered/
```

```diff
--- a/rendered/envs/mykso-prod/httpbingo/deployment-httpbingo.yaml
+++ b/rendered/envs/mykso-prod/httpbingo/deployment-httpbingo.yaml
@@ -11,7 +11,7 @@ metadata:
     helm.sh/chart: httpbingo-0.1.1
   name: httpbingo
 spec:
-  replicas: 1
+  replicas: 3
```

One field in one cluster. This is what a reviewer sees in the pull request.

The values file is named after the chart directory (`charts/httpbingo`), so one
application can render several charts, each with its own values. See
[Helm](helm.md) for the full lookup order.

## 5. Patch the output of a chart

Some changes cannot be made through chart values: a label the chart does not
expose, a field it does not template. Myks applies [ytt] overlays to the
rendered output. An overlay in `_env/ytt/` of an environment applies to every
application of that environment:

```shell
mkdir -p envs/mykso/prod/_env/ytt
cat > envs/mykso/prod/_env/ytt/labels.ytt.yaml <<'EOF'
#@ load("@ytt:overlay", "overlay")

#! Label every resource of every application in this environment.
#@overlay/match by=overlay.all, expects="1+"
---
metadata:
  #@overlay/match missing_ok=True
  labels:
    #@overlay/match missing_ok=True
    example.com/tier: production
EOF
myks render
git diff --stat rendered/
```

```text
 rendered/envs/mykso-prod/httpbingo/deployment-httpbingo.yaml     | 1 +
 rendered/envs/mykso-prod/httpbingo/service-httpbingo.yaml        | 1 +
 rendered/envs/mykso-prod/httpbingo/serviceaccount-httpbingo.yaml | 1 +
```

Overlays can also live in an application (`_apps/<app>/ytt/`) or in a prototype
(`prototypes/<proto>/ytt/`). See [ytt](ytt.md).

## 6. Pin a chart version for one cluster

The chart version is a data value of the prototype's vendir configuration, so an
environment can override it like any other value:

```shell
git add -A && git commit -m 'Label prod resources'
mkdir -p envs/mykso/prod/_apps/httpbingo/vendir
cat > envs/mykso/prod/_apps/httpbingo/vendir/vendir-data.ytt.yaml <<'EOF'
#@data/values
---
application:
  version: 0.1.0
EOF
myks render
git diff rendered/ | grep helm.sh/chart
```

```diff
-    helm.sh/chart: httpbingo-0.1.1
+    helm.sh/chart: httpbingo-0.1.0
```

Dev still renders 0.1.1. When you bump the version in the prototype, every
environment that does not pin it follows.

## 7. Find out where a value comes from

With several levels of overrides, `myks inspect` tells you which file sets each
final value:

```console
$ myks inspect apps mykso/prod httpbingo --provenance
...
  Helm Provenance (httpbingo):
    replicaCount envs/mykso/prod/_apps/httpbingo/helm/httpbingo.yaml
    securityContext.allowPrivilegeEscalation prototypes/httpbingo/helm/httpbingo.yaml
    ...
```

See [Inspecting and debugging](inspect.md).

## Deploying

Myks does not deploy. The generated Argo CD resources in `rendered/argocd/`
point Argo CD at `rendered/envs/<environment>/<application>/` in this
repository. Apply them once (or let an Argo CD "app of apps" pick them up), and
Argo CD deploys every commit you push. To deploy without Argo CD, set
`argocd.enabled: false` and use a [plugin](plugins.md), for example with kapp.

## Next steps

- [Concepts](concepts.md): the project layout, inheritance and render pipeline
  in detail.
- [Cookbook](cookbook.md): recipes for common tasks.
- [GitOps and CI](gitops-ci.md): render in pull requests and keep `rendered/` in
  sync.
- [Sources](sources.md): download charts and manifests from Git, OCI and private
  repositories.

[vendir]: https://carvel.dev/vendir/
[ytt]: https://carvel.dev/ytt/
