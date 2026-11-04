# Cookbook

Short recipes for common tasks. Each one names the file to create and links to
the page that explains the mechanism. The examples use a prototype `webapp`
rendering the `podinfo` chart, an application `frontend`, and the environment
tree `envs/` → `envs/prod/` → `envs/prod/eu/`.

- [Structure](#structure): [prototype](#add-a-prototype),
  [environment](#add-an-environment), [application](#add-an-application),
  [two instances](#run-a-prototype-twice-in-one-environment)
- [Values](#values):
  [one environment](#override-helm-values-for-one-environment),
  [a subtree](#change-every-application-of-a-prototype-in-a-subtree),
  [all charts](#share-values-between-the-charts-of-an-application),
  [from the environment](#derive-a-value-from-the-environment)
- [Sources](#sources): [pin a version](#pin-a-chart-version-in-one-environment),
  [unreleased chart](#try-an-unreleased-chart),
  [extra chart](#add-a-second-chart-to-one-application)
- [Manifests](#manifests): [patch](#patch-a-field-the-chart-does-not-expose),
  [remove](#remove-a-resource-from-a-chart),
  [add](#add-a-resource-next-to-a-chart),
  [whole environment](#change-every-application-of-an-environment)
- [Images](#images): [mirror registry](#pull-images-through-a-mirror)
- [Deployment](#deployment):
  [Argo CD](#change-the-generated-argo-cd-application),
  [secrets](#ship-encrypted-secrets),
  [Kubernetes version](#render-for-a-specific-kubernetes-version)

## Structure

### Add a prototype

A prototype is a directory in `prototypes/`; its name is the prototype name.
Most prototypes need a source and some defaults:

```text
prototypes/webapp/
├── app-data.ytt.yaml        # defaults and schema of application.*
├── vendir/base.ytt.yaml     # where the chart comes from
└── helm/podinfo.yaml        # default values of the podinfo chart
```

An empty directory is a valid prototype too: every file can come from the
application instead. See [Sources](sources.md) and
[Data values](data-values.md).

### Add an environment

Create a directory anywhere under `envs/` with an `env-data` file that sets
`environment.id`:

```yaml
#! envs/prod/us/env-data.ytt.yaml
#@data/values
---
environment:
  id: prod-us
  applications:
    - proto: webapp
      name: frontend
```

The directory inherits everything from `envs/prod/` and `envs/`. A directory
without `environment.id` is only an inheritance level. See
[Concepts](concepts.md#environments).

### Add an application

Append it to `environment.applications` at the level where it should run. To run
it on every production cluster, declare it in `envs/prod/env-data.ytt.yaml`; to
run it on one cluster, in that cluster's file. Lists are appended down the tree,
so both levels can declare applications.

### Run a prototype twice in one environment

Give each instance its own name:

```yaml
environment:
  applications:
    - proto: webapp
      name: frontend
    - proto: webapp
      name: frontend-canary
```

Each instance has its own `_apps/<name>/` directories, output directory and Argo
CD Application. The Helm release name defaults to the chart directory name and
the namespace to the application name, so the instances do not collide (see
[Helm](helm.md)).

## Values

### Override Helm values for one environment

Create a values file named after the chart directory in the application's
directory of that environment:

```yaml
#! envs/prod/eu/_apps/frontend/helm/podinfo.yaml
replicaCount: 3
```

Values files are ytt templates, so they can read data values:

```yaml
#@ load("@ytt:data", "data")
---
ui:
  message: #@ "Hello from " + data.values.environment.id
```

See [Helm](helm.md#values-files) for the full lookup order.

### Change every application of a prototype in a subtree

`_proto/<proto>/` works like `_apps/<app>/`, but applies to every application of
the prototype below that level:

```yaml
#! envs/prod/_proto/webapp/helm/podinfo.yaml
resources:
  requests:
    cpu: 100m
```

`_proto/` directories accept `helm/`, `ytt/`, `static/`, `lib/` and
`app-data*.yaml`.

### Share values between the charts of an application

Values in a file named `_global.yaml` (or `_global.<anything>.yaml`) apply to
every chart of the application, before the chart's own values files:

```yaml
#! prototypes/webapp/helm/_global.yaml
global:
  imageRegistry: mirror.example.com
```

### Derive a value from the environment

Data values files cannot read other data values, but they can read the final
values of their environment through `@myks:data.lib.yaml`:

```yaml
#! envs/_apps/frontend/app-data.ytt.yaml
#@ load("@myks:data.lib.yaml", "env_data")
#@data/values
---
application:
  host: #@ "shop." + env_data.environment.id + ".example.com"
```

See [ytt libraries](ytt-libraries.md#environment-data-library).

## Sources

### Pin a chart version in one environment

If the prototype templates its sources from data values (as `myks init`'s
prototypes do), override the value in the application's `vendir/` directory:

```yaml
#! envs/prod/_apps/frontend/vendir/vendir-data.ytt.yaml
#@data/values
---
application:
  version: 6.7.0
```

Otherwise overlay the vendir configuration. See
[Sources](sources.md#overriding-a-source-for-one-environment).

### Try an unreleased chart

Point the application at a Git branch with an overlay on its vendir
configuration, and set `lazy: false`. See
[Sources](sources.md#testing-an-unreleased-chart-from-a-branch).

### Add a second chart to one application

Overlay the vendir configuration to add a directory under `charts/`, and add a
values file named after it in `helm/`. See
[Sources](sources.md#overriding-a-source-for-one-environment).

## Manifests

These recipes use [ytt overlays](ytt.md). An overlay in `_apps/<app>/ytt/`
changes one application, in `prototypes/<proto>/ytt/` every application of the
prototype, and in `_env/ytt/` every application of an environment subtree.

### Patch a field the chart does not expose

```yaml
#! envs/prod/_apps/frontend/ytt/priority.ytt.yaml
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.subset({"kind": "Deployment", "metadata": {"name": "podinfo"}})
---
spec:
  template:
    spec:
      #@overlay/match missing_ok=True
      priorityClassName: high-priority
```

`#@overlay/match missing_ok=True` allows adding a key that the chart output does
not have.

### Remove a resource from a chart

Some charts create the namespace they deploy to, while Argo CD already creates
it:

```yaml
#! prototypes/webapp/ytt/no-namespace.ytt.yaml
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.subset({"kind": "Namespace"}), expects="0+"
#@overlay/remove
---
```

`expects="0+"` keeps the overlay from failing when there is nothing to remove.

### Add a resource next to a chart

Any plain YAML or ytt template in a `ytt/` directory becomes part of the output:

```yaml
#! prototypes/webapp/ytt/pdb.ytt.yaml
#@ load("@ytt:data", "data")
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: #@ data.values.myks.context.app
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: podinfo
```

An application without any chart works the same way: its manifests come only
from `ytt/` directories, or from plain YAML synced to `vendor/ytt/`
([Sources](sources.md#plain-manifests-from-git)).

### Change every application of an environment

```yaml
#! envs/prod/_env/ytt/labels.ytt.yaml
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.all, expects="1+"
---
metadata:
  #@overlay/match missing_ok=True
  labels:
    #@overlay/match missing_ok=True
    example.com/tier: production
```

`_env/ytt/` overlays run after each application's own ytt step. See
[ytt](ytt.md#where-myks-looks-for-files).

## Images

### Pull images through a mirror

Enable [kbld](kbld.md) and add an override. Overrides are regular expressions on
the registry, repository and tag of every image; the first match wins:

```yaml
#! envs/prod/env-data.kbld.yaml
#@data/values
---
kbld:
  enabled: true
  overrides:
    - match:
        registry: index\.docker\.io
      replace:
        registry: mirror.example.com
```

To give an application's override priority over inherited ones, insert it at the
start of the list:

```yaml
#! envs/prod/_apps/frontend/app-data.kbld.yaml
#@ load("@ytt:overlay", "overlay")
#@data/values
---
kbld:
  overrides:
    #@overlay/match by=overlay.index(0), missing_ok=True
    #@overlay/insert before=True
    - match:
        repository: stefanprodan/(.+)
      replace:
        registry: mirror.example.com
        repository: ghcr-mirror/stefanprodan/$1
```

## Deployment

### Change the generated Argo CD Application

Common fields are data values under `argocd.app`:

```yaml
#! envs/prod/_apps/frontend/app-data.argocd.yaml
#@data/values
---
argocd:
  app:
    syncPolicy:
      selfHeal: false
```

Other fields are set with an overlay in an `argocd/` directory. See
[Argo CD](argocd.md).

### Ship encrypted secrets

Put sops-encrypted files into a `static/` directory. Myks copies them to the
output unchanged, and the deployment tool decrypts them (for example Argo CD
with a sops plugin). See [Static files](static-files.md).

### Render for a specific Kubernetes version

Charts that check the cluster version or API availability need to know them at
render time:

```yaml
#! envs/prod/eu/env-data.helm.yaml
#@data/values
---
helm:
  kubeVersion: '1.31'
  capabilities:
    - monitoring.coreos.com/v1
```

See [Helm](helm.md#configuration).
