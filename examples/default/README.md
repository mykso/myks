# Default myks project

This directory contents is populated using the `myks init` command.

Included are two prototypes:

- Argocd: using the prerendered`manifests/ha/install.yaml` from argoCD github
  repo
- httpbingo: helm chart of small demo application and one environment
  (envs/mykso/dev).

## features

This example repository contains several aspects of overwrites myks can handle.

### application schema and configuration

ArgoCD is installed to the cluster from a plain manifest. The manifest is
changed during the rendering process.

- `prototypes/argocd/app-data.ytt.yaml` defines defaults and a schema
- The value of `gcpServiceAccountEmail` is overwritten in
  `envs/mykso/dev/_apps/argocd/app-data.ytt.yaml` only for the cluster `dev`
- the value is applied in `prototypes/argocd/ytt/argocd-vault-plugin.ytt.yaml`
  to add an annotation to the argoCD serviceAccount using ytt overlays

### general overlays

Overlays defined on root envs/\_env folder are applied to all environments.
Overlays can happen on every level:

- root (`envs`)
- group level (e.g. `envs/mykso/`)
- environment specific (`envs/mykso/dev`)

Example overlays:

- `envs/_env/argocd/annotations.overlay.ytt.yaml`: Overlay to apply to all
  argoCD resources. Adds annotations.
- `envs/_env/argocd/secret.overlay.ytt.yaml`: Extend the argoCD cluster
  definition (server URL and connect config)
- `envs/_env/ytt/common.ytt.yaml`: Overlay applied on all kubernetes resources

### multiple configurations levels

The httpbingo prototype defines it's own defaults
(`prototypes/httpbingo/helm/httpbingo.yaml`) on top of the helm chart defaults.
The replicaCount is overwritten for the dev cluster
(`envs/mykso/dev/_apps/httpbingo/helm/httpbingo.yaml`) with ytt support.

## tree

```python
.
├── envs
│   ├── _env
│   │   ├── argocd
│   │   │   ├── 'annotations.overlay.ytt.yaml' # adds annotation to all argo resources (rendered/argocd/**) using ytt
│   │   │   └── 'secret.overlay.ytt.yaml' # extends the argoCD cluster secret
│   │   └── ytt
│   │       └── 'common.ytt.yaml' # ytt overlay on all resources (common labels)
│   ├── 'env-data.ytt.yaml' # configures defaults for argoCD app and project
│   └── mykso
│       └── dev
│           ├── _apps
│           │   ├── argocd
│           │   │   ├── 'app-data.ytt.yaml' # set application value (gcp_sa) which is used in argocd-vault-plugin.ytt.yaml
│           │   │   └── vendor # vendored install.yaml
│           │   └── httpbingo
│           │       ├── argocd
│           │       │   └── 'overlay.ytt.yaml' # disable selfHeal for argoApp
│           │       ├── helm
│           │       │   └── 'httpbingo.yaml' # overwrite helm values
│           │       └── vendor # vendored helm chart
│           └── 'env-data.ytt.yaml' # define env and enabled applications
├── prototypes
│   ├── argocd
│   │   ├── 'app-data.ytt.yaml' # argoCD schema and defaults
│   │   ├── vendir # vendir source definition of upstream manifest
│   │   └── ytt
│   │       ├── 'argocd-vault-plugin.ytt.yaml' # extend installation: add annotation and enable vault plugin
│   │       └── 'ns.ytt.yaml' # create namespace resource for argoCD
│   ├── goldpinger
│   │   ├── helm
│   │   │   └── 'goldpinger.yaml' # helm default values for this prototype
│   │   ├── vendir # 2 vendir source definitions of helm chart AND grafana dashboards
│   │   └── ytt
│   │       └── 'grafana_dashboards.yaml' # load grafana dashboards (.json) into configmap
│   └── httpbingo
│       ├── helm
│       │   └── 'httpbingo.yaml' # helm default values for this prototype
│       └── vendir # vendir source definition of this helm chart
└── rendered
    ├── argocd
    │   └── mykso-dev # argocd definitions: Approject (env) and both applications
    │       ├── 'app-argocd.yaml'
    │       ├── 'app-httpbingo.yaml'
    │       └── 'env-mykso-dev.yaml'
    └── envs
        └── mykso-dev
            ├── argocd # rendered manifests argocd
            ├── goldpinger # rendered manifests goldpinger
            └── httpbingo # rendered manifests htttpbingo
```
