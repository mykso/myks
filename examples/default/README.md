# Default myks project

This project is what `myks init --argocd` creates, plus one extra prototype
(`goldpinger`) and a fixed Git URL and branch, so that the rendered output does
not depend on the checkout it runs in.

- Two environments, `envs/dev` and `envs/prod`, both running `httpbingo`, a
  small Helm chart.
- `dev` also runs Argo CD (the upstream `manifests/install.yaml`) and
  `goldpinger` (a Helm chart plus Grafana dashboards from a second source).
- Argo CD resources are generated for both environments into `rendered/argocd/`.

## Features

### Overlays at every level

Overlays in `envs/_env/` apply to every environment:

- `envs/_env/argocd/annotations.overlay.ytt.yaml`: adds annotations to every
  generated Argo CD resource.
- `envs/_env/argocd/secret.overlay.ytt.yaml`: fills in the Argo CD cluster
  secret (server URL and connection config).
- `envs/_env/ytt/common.ytt.yaml`: adds an annotation to every rendered
  Kubernetes resource.

`envs/dev/_apps/httpbingo/argocd/overlay.ytt.yaml` changes the Argo CD
`Application` of one application in one environment: it disables self-healing.

### Prototype defaults and patches

- `prototypes/argocd/ytt/ns.ytt.yaml` adds a `Namespace` and sets the namespace
  of every resource of the upstream Argo CD manifest.
- `prototypes/httpbingo/helm/httpbingo.yaml` sets Helm values on top of the
  chart defaults.
- `prototypes/goldpinger/vendir/` downloads two sources (the chart and the
  dashboards); `prototypes/goldpinger/ytt/grafana_dashboards.yaml` packs the
  dashboards into `ConfigMap`s.

## Tree

```text
.
├── envs
│   ├── _env
│   │   ├── argocd                      # overlays for the generated Argo CD resources
│   │   └── ytt/common.ytt.yaml         # overlay for every rendered resource
│   ├── env-data.ytt.yaml               # Argo CD defaults and the fixed Git settings
│   ├── dev
│   │   ├── _apps/httpbingo/argocd      # Argo CD Application overlay for one app
│   │   └── env-data.ytt.yaml           # id and applications of dev
│   └── prod
│       └── env-data.ytt.yaml           # id and applications of prod
├── prototypes
│   ├── argocd
│   ├── goldpinger
│   └── httpbingo
└── rendered
    ├── argocd
    │   ├── dev                         # AppProject, cluster Secret, one Application per app
    │   └── prod
    └── envs
        ├── dev                         # argocd, goldpinger, httpbingo
        └── prod                        # httpbingo
```
