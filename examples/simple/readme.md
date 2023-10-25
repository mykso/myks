# helm example

Simple example with two environments dev & prod with one helm chart.
- httpbingo helm chart version 0.1.0 is defined in the prototype
- dev environments overwrites the helm chart version (vendir config) (`envs/dev/_apps/httpbingo/vendir/vendir-data.ytt.yaml`)
- prod environments overwrites replica count (helm chart value) (`envs/prod/_apps/httpbingo/helm/httpbingo.yaml`)

## tree

```python
.
├── envs
│   ├── 'env-data.ytt.yaml' # shared environment configuration
│   ├── dev
│   │   ├── _apps
│   │   │   └── httpbingo
│   │   │       ├── vendir
│   │   │       │   └── 'vendir-data.ytt.yaml'  # overwrite helm chart version for dev environment
│   │   │       └── vendor # vendored helm chart
│   │   └── 'env-data.ytt.yaml' # environment configuration dev
│   └── prod
│       ├── _apps
│       │   └── httpbingo
│       │       ├── helm
│       │       │   └── 'httpbingo.yaml' # overwrite helm chart values for prod
│       │       └── vendor # vendored helm chart
│       └── 'env-data.ytt.yaml' # environment configuration prod
├── prototypes
│   └── httpbingo
│       ├── helm
│       │   └── 'httpbingo.yaml' # helm default values for all environments
│       └── vendir
│           ├── 'base.ytt.yaml' # templated vendir config
│           └── 'vendir-data.ytt.yaml' # vendir configuration, e.g. helm chart url and version (overwritten for dev)
└── rendered  # rendered files for all enviroments
    ├── argocd # argocd app definitionas
    │   ├── mykso-dev
    │   │   ├── 'app-httpbingo.yaml'
    │   │   └── 'env-mykso-dev.yaml'
    │   └── mykso-prod
    │       ├── 'app-httpbingo.yaml'
    │       └── 'env-mykso-prod.yaml'
    └── envs # rendered manifests
        ├── mykso-dev
        │   └── httpbingo
        │       ├── 'deployment-httpbingo.yaml'
        │       ├── 'service-httpbingo.yaml'
        │       └── 'serviceaccount-httpbingo.yaml'
        └── mykso-prod
            └── httpbingo
                ├── 'deployment-httpbingo.yaml'
                ├── 'service-httpbingo.yaml'
                └── 'serviceaccount-httpbingo.yaml'
```
