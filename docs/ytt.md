# ytt overlays

The ytt steps take everything rendered so far — Helm output, downloaded YAML,
your own files — and change it with [ytt](https://carvel.dev/ytt/) templates and
overlays. Use them to add resources, patch fields a chart does not expose, or
delete resources, without forking upstream sources. The changes live next to the
environment or application they apply to, and `git diff rendered/` shows their
effect.

There are two steps:

- **ytt** applies files of the application: its prototype, its `_proto/`
  overrides and its `_apps/` directories.
- **global-ytt** applies files of the environment (`_env/ytt/`) to every
  application of that environment.

Both run after Helm and before kbld, see the [render pipeline](concepts.md).
Their output replaces the previous output: an overlay can change or remove
anything earlier steps produced.

## Where myks looks for files

For the `frontend` application, based on the `webapp` prototype, in the
`envs/prod/eu` environment, the **ytt** step reads, in this order:

1. `vendor/ytt/` — files downloaded by vendir into `ytt/`, see
   [sources.md](sources.md)
2. `prototypes/webapp/ytt/`
3. `envs/_proto/webapp/ytt/`
4. `envs/prod/_proto/webapp/ytt/`
5. `envs/prod/eu/_proto/webapp/ytt/`
6. `envs/_apps/frontend/ytt/`
7. `envs/prod/_apps/frontend/ytt/`
8. `envs/prod/eu/_apps/frontend/ytt/`

The **global-ytt** step then reads:

1. `envs/_env/ytt/`
2. `envs/prod/_env/ytt/`
3. `envs/prod/eu/_env/ytt/`

```
.
├── prototypes/webapp/ytt/           # ytt 2: every application of the prototype
└── envs/
    ├── _env/ytt/                    # global-ytt 1: every application everywhere
    ├── _proto/webapp/ytt/           # ytt 3
    ├── _apps/frontend/ytt/          # ytt 6
    └── prod/
        ├── _env/ytt/                # global-ytt 2: every application in prod
        └── eu/
            ├── _env/ytt/            # global-ytt 3: every application in prod-eu
            └── _apps/frontend/ytt/  # ytt 8: frontend in prod-eu only
```

ytt processes all files of a step in one run. Files without ytt annotations and
templates add documents; overlays apply in the order above, so an overlay of a
more specific directory sees the result of the more general ones.

## What a step receives

Besides the files above, myks passes to both steps:

- the output of the previous step (`.myks/<env-dir>/_apps/<app>/steps/`);
- the built-in data-values schema (`.myks/tmp/data-schema.ytt.yaml`);
- git data (`.myks/tmp/myks-data.ytt.yaml`: `myks.gitRepoUrl`,
  `myks.gitRepoBranch`);
- all [data-values files](data-values.md) of the application, so
  `data.values.application`, `data.values.environment` and the other scopes are
  available;
- ytt libraries: the root `lib/`, the environment data library
  (`@myks:data.lib.yaml`) and the application and prototype `lib/` directories,
  see [ytt-libraries.md](ytt-libraries.md);
- context values: `myks.context.app` (application name),
  `myks.context.prototype` (prototype path, for example `prototypes/webapp`) and
  `myks.context.step` (`render-ytt` or `global-ytt`).

```yaml
#@ load("@ytt:data", "data")
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: #@ data.values.myks.context.app + "-info"
data:
  environment: #@ data.values.environment.id
```

> [!TIP]  
> `myks render <env> <app> --render --log-level debug` prints the full `ytt`
> command of each step, and `.myks/<env-dir>/_apps/<app>/steps/` keeps each
> step's output. See [inspect.md](inspect.md#debugging-a-render).

## Examples

### Add a resource

Any plain YAML file in a `ytt/` directory adds its documents to the output:

```yaml
#! envs/prod/eu/_apps/frontend/ytt/pdb.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: frontend
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: frontend
```

### Set a field the chart does not expose

```yaml
#! prototypes/webapp/ytt/deployment.ytt.yaml
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.subset({"kind": "Deployment"}), expects="1+"
---
spec:
  #@overlay/match missing_ok=True
  revisionHistoryLimit: 3
```

### Remove a resource

```yaml
#! envs/prod/_apps/frontend/ytt/no-hpa.ytt.yaml
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.subset({"kind": "HorizontalPodAutoscaler"})
#@overlay/remove
---
```

### Label everything in an environment

```yaml
#! envs/prod/_env/ytt/labels.ytt.yaml
#@ load("@ytt:data", "data")
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.all, expects="0+"
---
#@overlay/match-child-defaults missing_ok=True
metadata:
  labels:
    example.com/environment: #@ data.values.environment.id
```

> [!IMPORTANT]  
> An overlay in `_env/ytt/` runs for every application of the environment, and
> most applications do not contain the resource it matches. ytt expects exactly
> one match by default and fails otherwise:
>
> ```text
> Expected number of matched nodes to be 1, but was 0
> ```
>
> Add `expects="0+"` (or `"1+"` where a match is required) to every
> `#@overlay/match` in `_env/ytt/`.

## ytt packages

The **ytt-pkg** step, which runs right after Helm, renders downloaded ytt
packages in isolation, the way Helm renders charts. Every directory under the
application's `vendor/ytt-pkg/` is one package. It is rendered by ytt on its own
— without myks data values or libraries — with a data-values file built from
`ytt-pkg/<package>.*yaml` (and `ytt-pkg/_global.*yaml`) files, which are
collected and merged exactly like [Helm values files](helm.md#values-files).
`yttPkg.dirs` lists sub-paths of each package to pass to ytt instead of the
whole directory.

Use it for upstream configuration distributed as a ytt package with its own data
values. To modify YAML you download, put it under `ytt/` instead: the ytt step
renders it together with your overlays and data values.
