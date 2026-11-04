# CLI reference

This page lists every myks command, how the positional selectors pick
environments and applications, and every global flag. Run
`myks <command> --help` for the same information in the terminal.

## Commands

| Command                 | What it does                                                                                              |
| ----------------------- | --------------------------------------------------------------------------------------------------------- |
| `render`                | Syncs external sources and renders manifests. See [render](#render).                                      |
| `init`                  | Creates a new project in the current directory. See [init](#init).                                        |
| `inspect`               | Shows environments, applications, prototypes, data values and their origin. See [inspect.md](inspect.md). |
| `cleanup`               | Removes rendered output and cache entries that no environment uses. See [cleanup](#cleanup).              |
| `migrate`               | Converts a ytt data-values project to the KCL configuration layer. See [migration.md](migration.md).      |
| `print-config`          | Prints the effective configuration: `.myks.yaml`, `MYKS_*` variables and flags merged.                    |
| `completion`            | Generates a shell completion script (`bash`, `zsh`, `fish`, `powershell`).                                |
| `ytt`, `vendir`, `kbld` | Runs the embedded copy of the tool, for example `myks ytt -f config.yaml`.                                |
| `<plugin>`              | Runs a plugin for the selected applications. See [plugins.md](plugins.md).                                |

`myks --version` prints the version, commit and build date.

## Selecting environments and applications

`render`, `inspect apps` and plugin commands take up to two positional
arguments:

```shell
myks render [environments [applications]]
```

- **environments** is a comma-separated list. Each item is a directory path
  under `envs/` (the `envs/` prefix is optional) and selects **every environment
  at or below that directory**. `myks render prod` processes `envs/prod/eu`,
  `envs/prod/us` and any other environment under `envs/prod`. An item can also
  be an environment ID (`prod-eu`) once the environment has been rendered: myks
  finds `rendered/envs/prod-eu` and resolves it back to its directory. In
  [KCL mode](kcl-mode.md) IDs always resolve.
- **applications** is a comma-separated list of application names (not prototype
  names). When omitted, all applications of the selected environments are
  processed.
- `ALL` in either position means "all".
- **No arguments** turns on [Smart Mode](smart-mode.md) for `render` and plugin
  commands: myks processes only what your Git changes affect. `inspect` and
  `cleanup` never use Smart Mode; without arguments they cover everything.

```shell
myks render                          # Smart Mode
myks render ALL                      # every application in every environment
myks render prod                     # everything under envs/prod
myks render prod/eu,stage frontend   # one application in two subtrees
myks render ALL frontend,backend     # two applications wherever they are deployed
```

> [!TIP]  
> Shell completion knows both forms: directory paths under `envs/` and the IDs
> of rendered environments for the first argument, application names for the
> second. Install it with `myks completion <shell> --help`.

## render

```shell
myks render [environments [applications]] [--sync | --render]
```

Runs both stages for the selected applications: **sync** downloads sources with
vendir and builds Helm dependencies, **render** runs the
[render pipeline](concepts.md) and writes the result to `rendered/`.

| Flag           | Effect                                                                               |
| -------------- | ------------------------------------------------------------------------------------ |
| `-s, --sync`   | Only sync sources.                                                                   |
| `-r, --render` | Only render, reusing the sources of the last sync. Mutually exclusive with `--sync`. |

With `--render`, myks warns about applications whose sources look unsynced (no
sync has run, or a vendored directory is missing), because they may render with
missing sources.

All selected applications are processed even when some fail; the command exits
non-zero if any of them failed.

After processing, myks deletes the rendered output
(`rendered/envs/<env-id>/<app>/` and `rendered/argocd/<env-id>/app-<app>.yaml`)
of applications that are no longer listed in a processed environment.
`myks render ALL` (a single `ALL`) also deletes the rendered output of
environments that no longer exist; `myks render ALL ALL` and Smart Mode runs do
not. Use [`cleanup`](#cleanup) to do it explicitly.

Credentials for private sources are passed with `VENDIR_SECRET_*` environment
variables, see [sources.md](sources.md).

## init

```shell
myks init [--force] [--print] [--components <list>]
```

Creates `envs/`, `prototypes/`, `rendered/envs/`, `.gitignore` and `.myks.yaml`
in the current directory, plus a sample environment and two sample prototypes
(Argo CD and a Helm chart). It refuses to overwrite any of these unless you pass
`--force`.

| Flag                  | Effect                                                                                                                                                      |
| --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `-f, --force`         | Overwrite existing files.                                                                                                                                   |
| `--print`             | Print files instead of writing them. Applies to the `config`, `gitignore` and `schema` components.                                                          |
| `--components <list>` | Comma-separated subset of `config`, `environments`, `gitignore`, `prototypes`, `schema`. Default: all. `environments` and `prototypes` control the samples. |

`myks init --print --components schema` prints the built-in data-values schema,
which is the reference for every myks setting (`argocd`, `helm`, `kbld`,
`render`, `yttPkg`). `myks init --print --components config` prints a
`.myks.yaml` with all [naming conventions](configuration.md#naming-conventions)
spelled out.

`init` scaffolds a ytt data-values project. See [kcl-mode.md](kcl-mode.md) for
KCL projects.

## cleanup

```shell
myks cleanup [--manifests] [--cache] [--dry-run]
```

| Flag          | Effect                                                                                 |
| ------------- | -------------------------------------------------------------------------------------- |
| `--manifests` | Delete rendered output of environments and applications that are no longer configured. |
| `--cache`     | Delete entries in `.myks/vendir-cache/` that no application links to.                  |
| `--dry-run`   | Log what would be deleted, delete nothing.                                             |

Without `--manifests` and `--cache`, both run. `cleanup` always considers all
environments and takes no positional arguments. See
[optimizations.md](optimizations.md#centralized-cache).

## print-config

Prints the configuration myks runs with, after merging `.myks.yaml`, `MYKS_*`
environment variables and flags. Useful to check which config file and root
directory are picked up.

## Global flags

Every flag can also be set in `.myks.yaml` (same key) or as an environment
variable: prefix `MYKS_`, upper case, `.` and `-` replaced with `_`. Flags win
over environment variables, which win over the config file. See
[configuration.md](configuration.md).

| Flag                               | Environment variable            | Default | Effect                                                                                                                                             |
| ---------------------------------- | ------------------------------- | ------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `-a, --async <n>`                  | `MYKS_ASYNC`                    | `0`     | Applications processed in parallel; `0` means no limit. Applies to sync, render, `inspect` and plugins.                                            |
| `-l, --log-level <level>`          | `MYKS_LOG_LEVEL`                | `info`  | `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`. `debug` logs every executed command, see [inspect.md](inspect.md#debugging-a-render). |
| `--smart-mode.base-revision <rev>` | `MYKS_SMART_MODE_BASE_REVISION` | none    | Git revision Smart Mode compares the working tree with. Without it, only uncommitted changes count. See [smart-mode.md](smart-mode.md).            |
| `--smart-mode.only-print[=fmt]`    | `MYKS_SMART_MODE_ONLY_PRINT`    | off     | Print what Smart Mode (or the given selectors) would process and exit. `text` (default without a value) or `json`.                                 |
| `--buffer-plugin-output`           | `MYKS_BUFFER_PLUGIN_OUTPUT`     | `false` | Collect each plugin run's stdout and stderr and print them when it finishes, instead of streaming. Keeps output of parallel plugin runs apart.     |
| `--print-stats`                    | `MYKS_PRINT_STATS`              | `false` | Print the run summary (tool calls, timings, memory, parallelism, lock contention, sync deduplication) to stdout instead of the log on stderr.      |
| `--config <file>`                  | —                               | none    | Config file to use instead of the first `.myks.yaml` found in the current directory or its parents.                                                |

Set `NO_COLOR` to disable colored output; it is also disabled when stdout is not
a terminal.
