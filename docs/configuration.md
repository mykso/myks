# Configuration reference

`.myks.yaml` holds project-wide settings for the myks CLI: parallelism, logging,
plugin directories, and the names of the files and directories myks recognizes.
`myks init` creates one with every option spelled out. Settings for rendering
itself — Helm flags, Argo CD fields, image overrides — are not here: they are
data values, see [data-values.md](data-values.md).

## Where myks finds the file

myks uses the first `.myks.yaml` it finds in the current directory or any parent
directory. With `config-in-root: true`, the directory of that file becomes the
project root, so you can run myks from any subdirectory. `--config <file>`
skips the search and uses the given file.

Run `myks print-config` to see the settings myks ends up with. The log line
`Using config file: ...` at the start of every run names the file.

## Precedence

Every top-level option can also be set as an environment variable (`MYKS_`
prefix, upper case, `-` and `.` replaced with `_`) and most as a
[command-line flag](cli.md#global-flags). A flag wins over an environment
variable, which wins over the file:

```shell
MYKS_ASYNC=4 myks render         # same as `async: 4` in .myks.yaml
MYKS_LOG_LEVEL=debug myks render # same as --log-level debug
```

## Options

### `async`

- **Type**: integer
- **Default**: `0` (no limit)
- **Environment variable**: `MYKS_ASYNC`
- **Flag**: `--async`, `-a`

Number of applications processed in parallel during sync, render, `inspect` and
plugin runs.

### `config-in-root`

- **Type**: boolean
- **Default**: `false` (`myks init` writes `true`)
- **Environment variable**: `MYKS_CONFIG_IN_ROOT`

Makes the directory of the config file the project root, so `myks` works from
any subdirectory of the project.

### `log-level`

- **Type**: string
- **Default**: `info`
- **Values**: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`
  ([zerolog levels](https://github.com/rs/zerolog#leveled-logging))
- **Environment variable**: `MYKS_LOG_LEVEL`
- **Flag**: `--log-level`, `-l`

`debug` logs every command myks runs, see
[inspect.md](inspect.md#debugging-a-render).

### `min-version`

- **Type**: string
- **Default**: none
- **Environment variable**: `MYKS_MIN_VERSION`

Minimum myks version the project needs, for example `v5.16.0`. An older myks
logs an error at startup and continues.

### `plugin-sources`

- **Type**: list of strings
- **Default**: none (`myks init` writes `['./plugins']`)
- **Environment variable**: `MYKS_PLUGIN_SOURCES` (space-separated)

Directories whose executables become myks [plugins](plugins.md), whatever their
names. Environment variables in the paths are expanded (`$HOME/myks-plugins`).

> [!IMPORTANT]  
> Relative paths are resolved against the current working directory, not the
> project root. With `config-in-root: true`, running myks from a subdirectory
> does not find `./plugins`; use an absolute path or run myks from the root.

### `root-dir`

- **Type**: string
- **Default**: `.`
- **Environment variable**: `MYKS_ROOT_DIR`

Project root. myks changes into this directory before it starts. Set
automatically when `config-in-root` is `true`.

### `naming-conventions`

Names of the files and directories myks recognizes, for projects that follow
other conventions. Paths are relative to the project root.

| Key                          | Default           | What it names                                                         |
| ---------------------------- | ----------------- | --------------------------------------------------------------------- |
| `environment-base-dir`       | `envs`            | Root of the environment tree                                          |
| `prototypes-dir`             | `prototypes`      | Prototypes directory                                                  |
| `ytt-library-dir-name`       | `lib`             | ytt library directory, at the root and in prototypes and applications |
| `rendered-envs-dir`          | `rendered/envs`   | Output directory for manifests                                        |
| `rendered-argo-dir`          | `rendered/argocd` | Output directory for Argo CD resources                                |
| `apps-dir`                   | `_apps`           | Application directories inside an environment                         |
| `envs-dir`                   | `_env`            | Environment-wide directory inside an environment                      |
| `prototype-override-dir`     | `_proto`          | Prototype override directories inside an environment                  |
| `application-data-file-name` | `app-data*.yaml`  | Glob of application data-values files                                 |
| `environment-data-file-name` | `env-data*.yaml`  | Glob of environment data-values files                                 |
| `plugin-argocd-dir-name`     | `argocd`          | Argo CD overlays directory                                            |
| `plugin-helm-dir-name`       | `helm`            | Helm values directory                                                 |
| `plugin-static-dir-name`     | `static`          | Static files directory                                                |
| `plugin-vendir-dir-name`     | `vendir`          | vendir configuration directory                                        |
| `plugin-ytt-dir-name`        | `ytt`             | ytt templates and overlays directory                                  |

```yaml
naming-conventions:
  environment-base-dir: clusters
  apps-dir: apps
```

The rest of the layout is fixed: the `.myks/` service directory, `charts/` and
`ytt-pkg/` inside the vendor directory, the `ytt-pkg/` values directory and the
`VENDIR_SECRET_` variable prefix. `myks init` ignores `naming-conventions` and
always scaffolds the default layout.

## Smart Mode options

Smart Mode is controlled by flags or environment variables; see
[smart-mode.md](smart-mode.md) for what it does.

### `smart-mode.base-revision`

- **Type**: string
- **Default**: none (only uncommitted changes count)
- **Environment variable**: `MYKS_SMART_MODE_BASE_REVISION`
- **Flag**: `--smart-mode.base-revision`

Git revision to compare the working tree with.

```shell
myks render --smart-mode.base-revision=main
myks render --smart-mode.base-revision=HEAD~1
```

### `smart-mode.only-print`

- **Type**: string
- **Default**: `''` (disabled)
- **Values**: `text` (default when the flag has no value), `json`. `true`, `1`,
  `yes` mean `text`; `false`, `0`, `no` mean disabled.
- **Environment variable**: `MYKS_SMART_MODE_ONLY_PRINT`
- **Flag**: `--smart-mode.only-print`

Prints the environments and applications that would be processed, then exits.

```shell
myks render --smart-mode.only-print
myks render --smart-mode.only-print=json
```

## Example

```yaml
async: 4
config-in-root: true
log-level: info
min-version: 'v5.16.0'
plugin-sources:
  - ./plugins
naming-conventions:
  environment-base-dir: envs
  prototypes-dir: prototypes
```
