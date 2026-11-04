---
title: Static files
sidebar:
  order: 7
---

The static step copies files to the rendered output of an application as they
are: no templating, no slicing into one file per resource, no image resolution.
Use it for content that must reach the deployment tool unchanged, for example
[sops](https://github.com/getsops/sops)-encrypted secrets that Argo CD decrypts
with a plugin at sync time, or files with `#@` lines that ytt would otherwise
interpret.

## Where myks looks for files

For the `frontend` application, based on the `webapp` prototype, in the
`envs/prod/eu` environment, myks copies the content of these directories, in
this order:

1. `prototypes/webapp/static/`
2. `envs/_proto/webapp/static/`
3. `envs/prod/_proto/webapp/static/`
4. `envs/prod/eu/_proto/webapp/static/`
5. `envs/_env/static/`
6. `envs/prod/_env/static/`
7. `envs/prod/eu/_env/static/`
8. `envs/_apps/frontend/static/`
9. `envs/prod/_apps/frontend/static/`
10. `envs/prod/eu/_apps/frontend/static/`

```
.
├── prototypes/webapp/static/            # 1: every application of the prototype
└── envs/
    ├── _env/static/                     # 5: every application everywhere
    ├── _apps/frontend/static/           # 8
    └── prod/
        └── eu/
            ├── _env/static/             # 7: every application in prod-eu
            └── _apps/frontend/static/   # 10: frontend in prod-eu only
```

## Output

Files land in the `static/` subdirectory of the application's output, keeping
their relative paths:

```
envs/prod/eu/_apps/frontend/static/secrets/db.sops.yaml
→ rendered/envs/prod-eu/frontend/static/secrets/db.sops.yaml
```

A file with the same relative path in a later directory of the list replaces the
earlier one, so an environment can override a prototype's file by placing a file
with the same name.

The step runs after the manifests are sliced into
`rendered/envs/<env-id>/<app>/` and before the Argo CD step. It copies files
from your repository only: sources downloaded by vendir are not read by this
step.
