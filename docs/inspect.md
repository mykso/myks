# Inspecting and debugging

`myks inspect` answers "where does this come from?" without running a render:
which environments and applications exist, which files feed each step of an
application, what the final data values are, and which file sets each of them.
Use it when a value is not what you expect, before refactoring the inheritance
tree, and in CI to keep the tree free of redundant overrides.

`inspect` resolves configuration and data values but runs no sync, Helm, ytt
overlay, kbld or Argo CD step. It writes intermediate files to `.myks/`, like
any other command.

The outputs below come from
[`examples/integration-tests`](../examples/integration-tests).

## Environments

```shell
myks inspect envs [env-selector] [--data-values] [-o json]
```

```console
$ myks inspect envs
Environment: mykso-dev
  Dir:      envs/dev
  Rendered: rendered/envs/mykso-dev
  ArgoCD:   rendered/argocd/mykso-dev
  Config:
    - envs/env-data.ytt.yaml
    - envs/dev/env-data.ytt.yaml
  Applications:
    argocd-tests                   prototype: ytt-render-test
    helm-installation              prototype: helm-render-test
    static-test                    prototype: static-test
    ...
```

`Config` lists the environment data files in merge order, from the root of the
tree to the environment. `--data-values` adds the merged environment data.

## Applications

```shell
myks inspect apps [env-selector [app-selector]] [flags]
```

The selectors work as for `render`, see
[cli.md](cli.md#selecting-environments-and-applications). Without them, every
application of every environment is shown.

```console
$ myks inspect apps dev helm-installation
Application: helm-installation

  Environment: mykso-dev (envs/dev)
  Prototype:  helm-render-test
  Rendered:   rendered/envs/mykso-dev/helm-installation
  Service:    .myks/envs/dev/_apps/helm-installation

  Common Files:
    - lib
    - .myks/tmp/data-schema.ytt.yaml
    - .myks/tmp/myks-data.ytt.yaml
    - .myks/envs/dev/_api
    - envs/env-data.ytt.yaml
    - envs/dev/env-data.ytt.yaml
    - prototypes/helm-render-test/app-data.ytt.yaml
    ...

  Step Files:
    sync-vendir:
      - prototypes/helm-render-test/vendir
    render-helm:
      - prototypes/helm-render-test/helm/render-test.bar.yaml
      - prototypes/helm-render-test/helm/render-test.foo.yaml
      - prototypes/helm-render-test/helm/render-test.yaml
      - envs/_proto/helm-render-test/helm/render-test.baz.yaml
      - envs/_proto/helm-render-test/helm/render-test.yaml
      - envs/dev/_proto/helm-render-test/helm/render-test.yaml
      - envs/_apps/helm-installation/helm/render-test.quux.yaml
      - envs/_apps/helm-installation/helm/render-test.yaml
      - envs/dev/_apps/helm-installation/helm/render-test.yaml
```

- **Common Files** are passed to every ytt call of the application: the root
  `lib/`, the built-in schema, git data, the environment data library, and the
  data-values files in merge order.
- **Step Files** are the step-specific inputs in the order they apply. Only
  steps that have inputs are listed.

| Flag                | Adds                                                                                                                               |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `--data-values`     | The final data values of the application.                                                                                          |
| `--provenance`      | For every final data value, the file that sets it; for every Helm (and ytt-pkg) values key, the values file that sets it.          |
| `--provenance=all`  | Like `--provenance`, and also lists the values that come from the built-in myks defaults (text output; JSON always contains them). |
| `--redundant`       | Values that a file sets to exactly what it already inherits. Exits non-zero when it finds any.                                     |
| `--rendered`        | The artifacts in `.myks/` left by the last render: `vendir.yaml`, merged Helm values files, step outputs.                          |
| `-o, --output json` | Machine-readable output.                                                                                                           |

### Where does a value come from?

```console
$ myks inspect apps dev helm-installation --provenance
  ...
  Provenance:
    application.appValue envs/dev/_apps/helm-installation/app-data.ytt.yaml
    application.baseValue prototypes/helm-render-test/app-data.ytt.yaml
    application.envGroupValue envs/_apps/helm-installation/app-data.ytt.yaml
    application.protoOverwriteValue envs/_proto/helm-render-test/app-data.ytt.yaml
    argocd.namespace envs/env-data.ytt.yaml
    environment.id envs/dev/env-data.ytt.yaml
    ...

  Helm Provenance (render-test):
    outputYaml.extraValuesAppQuux envs/_apps/helm-installation/helm/render-test.quux.yaml
    outputYaml.fromAppConfig envs/dev/_apps/helm-installation/helm/render-test.yaml
    outputYaml.fromPrototype prototypes/helm-render-test/helm/render-test.yaml
    ...
```

The file named is the last one in merge order that sets the value, so it is the
file to edit.

Some defaults are filled in only when a step runs, for example the Argo CD
application name and source path (see [argocd.md](argocd.md#configuration)).
They show as empty strings in `--data-values`.

### Finding redundant overrides

An override that repeats the inherited value changes nothing today, but it pins
the value: a later change at the upper level silently stops reaching this
environment. `--redundant` lists such overrides together with the file they
repeat:

```console
$ myks inspect apps dev helm-installation --redundant
  ...
  Helm Redundant Overrides (render-test):
    kept in envs/_proto/helm-render-test/helm/render-test.yaml
    repeated in envs/dev/_proto/helm-render-test/helm/render-test.yaml
      outputYaml.fromPrototypeOverride = true
      outputYaml.fromPrototypeOverrideFromAppData = true
Error: found 2 redundant overrides
```

The command exits with status 1 when it finds any, so it works as a CI check:

```shell
myks inspect apps --redundant > /dev/null
```

`--redundant` also catches overrides that do not do what they look like. In ytt
data values an empty list does not clear an inherited list, so an override such
as `argocd.app.finalizers: []` leaves the built-in default in place and shows up
as redundant. Put `#@overlay/replace` above the key to replace a list.

## Prototypes

```shell
myks inspect prototypes [name] [-o json]
```

Lists every prototype and the applications that use it, across all environments:

```console
$ myks inspect prototypes ytt-render-test
Prototype: ytt-render-test
  Dir: prototypes/ytt-render-test
  Used by:
    argocd-tests                   in mykso-dev (envs/dev)
    ytt-installation               in mykso-dev (envs/dev)
```

## Debugging a render

### See every command

`--log-level debug` logs each command myks runs, with all its arguments. That
shows exactly which files reach a step and in which order:

```console
$ myks render dev ytt-installation --render --log-level debug
DBG [mykso-dev > ytt-installation > render-ytt] Ran myks to: render local ytt
myks ytt --file=lib --file=.myks/tmp/data-schema.ytt.yaml --file=.myks/tmp/myks-data.ytt.yaml
  --file=.myks/envs/dev/_api --file=envs/env-data.ytt.yaml --file=envs/dev/env-data.ytt.yaml
  --file=prototypes/ytt-render-test/app-data.ytt.yaml ...
  --file=.myks/envs/dev/_apps/ytt-installation/steps/01-ytt-pkg.yaml
  --file=prototypes/ytt-render-test/ytt ... -v myks.context.step=render-ytt ...
```

`myks ytt`, `myks vendir` and `myks kbld` run the embedded tools, so you can
copy such a command, change it, and run it by hand. `--log-level trace` adds
more detail, for example which Smart Mode patterns matched which file.

### The `.myks/` service directory

Every intermediate result is kept in `.myks/`. It is safe to delete; the next
sync and render recreate it.

```
.myks/
├── tmp/
│   ├── data-schema.ytt.yaml          # built-in data-values schema
│   └── myks-data.ytt.yaml            # git branch and repository URL
├── vendir-cache/                     # downloaded sources, shared by all applications
│   └── <source>-<hash>/
└── envs/<env-dir>/
    ├── _api/_ytt_lib/myks/data.lib.yaml   # environment data library (@myks:data.lib.yaml)
    └── _apps/<app>/
        ├── vendir.yaml               # rendered vendir config
        ├── vendir-links.yaml         # vendor path → cache entry
        ├── vendor/                   # symlinks into vendir-cache: charts/, ytt/, ...
        ├── helm/
        │   ├── unmerged_<chart>.yaml # values files rendered by ytt, one document each
        │   └── <chart>.yaml          # merged values passed to helm template
        ├── argocd_defaults.ytt.yaml  # ArgoCD defaults computed for this application
        └── steps/                    # output after each render step
            ├── 00-helm.yaml
            ├── 01-ytt-pkg.yaml
            ├── 02-ytt.yaml
            ├── 03-global-ytt.yaml
            └── 04-kbld.yaml
```

The step outputs are cumulative: `00-helm.yaml` holds the Helm output,
`02-ytt.yaml` holds everything after the ytt overlays, `04-kbld.yaml` is what
gets sliced into `rendered/envs/<env-id>/<app>/`. Diff two of them to see what a
step changed:

```shell
diff .myks/envs/prod/eu/_apps/frontend/steps/00-helm.yaml \
     .myks/envs/prod/eu/_apps/frontend/steps/03-global-ytt.yaml
```

### Run statistics

At the end of every run, myks logs a summary: calls and timing per tool and
step, parallelism achieved, lock contention, and how many vendir syncs were
skipped thanks to the cache. `--print-stats` writes the same summary to stdout
instead of the log:

```console
$ myks render ALL --print-stats
...
--- Pipeline Concurrency Summary ---
Apps processed:      9
Async level:         unlimited
Wall clock time:     479ms
Sequential estimate: 3.01s
Speedup:             6.28x

--- Vendir Sync Dedup Summary ---
Syncs executed:          7
Skipped (in-run dedup):  5
Skipped (cached):        0
```
