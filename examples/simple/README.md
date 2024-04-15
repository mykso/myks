# Simple example

This example project consists of two environments (`dev` and `prod`) with a
single helm chart.

- the single helm chart `httpbingo` of version `0.1.0` is defined in the
  `prototypes` directory
- the `dev` environment overwrites the version of the helm chart via the vendir
  data values file in
  [`envs/dev/_apps/httpbingo/vendir/vendir-data.ytt.yaml`](envs/dev/_apps/httpbingo/vendir/vendir-data.ytt.yaml)
- the `prod` environment overwrites the replica count via the helm chart values
  file in
  [`envs/prod/_apps/httpbingo/helm/httpbingo.yaml`](envs/prod/_apps/httpbingo/helm/httpbingo.yaml)

## File tree

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
