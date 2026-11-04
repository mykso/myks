# Plugins

A plugin is any executable that myks runs once per application, after rendering,
with the application's context in environment variables. Use plugins to act on
the rendered output with other tools, for example:

- validate manifests with `kubeconform`, `conftest` or `promtool`;
- deploy without a GitOps controller, with `kubectl` or `kapp`;
- decrypt secrets before applying them and re-encrypt them afterwards.

A plugin cannot change how myks renders: it runs as a separate command and reads
what `myks render` wrote.

## Installing a plugin

myks loads plugins from two places:

- **`PATH`**: executables named `myks-<name>` become the `myks <name>` command.
- **`plugin-sources`**: every executable in the directories listed in
  [`.myks.yaml`](configuration.md#plugin-sources), whatever its name.
  `myks init` configures `./plugins`.

```yaml
plugin-sources:
  - ./plugins
  - $HOME/myks-plugins
```

The command name is the file name without the `myks-` prefix (for `PATH`) and
without its extension: `plugins/validate.sh` becomes `myks validate`. When two
plugins have the same name, the one from `PATH` wins and myks logs a warning.
Plugins are listed under **Plugin Subcommands** in `myks help`.

## Running a plugin

```shell
myks <plugin> [environments [applications]] [-- <plugin-args>...]
```

The positional arguments select applications the same way as for `render`, see
[cli.md](cli.md#selecting-environments-and-applications). Without them,
[Smart Mode](smart-mode.md) selects the applications affected by your changes,
including those whose rendered output changed, so `myks render && myks validate`
checks exactly what was just rendered. Everything after `--` is passed to the
plugin as arguments.

```shell
myks validate                          # applications affected by local changes
myks validate prod ALL                 # every application under envs/prod
myks validate prod/eu frontend -- -verbose
```

myks runs the plugin once per selected application, in parallel up to
[`--async`](configuration.md#async). It runs every selected application even
when some fail, and exits non-zero if any run failed.

By default plugin output is streamed as it comes, so the output of parallel runs
interleaves. `--buffer-plugin-output` collects each run's stdout and stderr and
prints them when the run finishes.

## Environment variables

Paths are relative to the project root, which is also the plugin's working
directory.

| Variable                | Content                                                         | Example                          |
| ----------------------- | --------------------------------------------------------------- | -------------------------------- |
| `MYKS_ENV`              | Environment ID                                                  | `prod-eu`                        |
| `MYKS_ENV_DIR`          | Environment directory                                           | `envs/prod/eu`                   |
| `MYKS_APP`              | Application name                                                | `frontend`                       |
| `MYKS_APP_PROTOTYPE`    | Prototype directory                                             | `prototypes/webapp`              |
| `MYKS_RENDERED_APP_DIR` | Rendered output of the application                              | `rendered/envs/prod-eu/frontend` |
| `MYKS_DATA_VALUES`      | Final [data values](data-values.md) of the application, as YAML |                                  |

`MYKS_DATA_VALUES` lets a plugin take its settings from the configuration tree:
define them under a scope of your own, give them defaults at the root, and
override them per environment or application like any other value.

## Example: validate manifests with kubeconform

`plugins/validate`:

```bash
#!/usr/bin/env bash
# Validates the rendered manifests of one application against Kubernetes schemas.
set -euo pipefail

kube_version=$(yq '.helm.kubeVersion // ""' <<<"$MYKS_DATA_VALUES")
args=(-strict -summary)
if [[ -n $kube_version ]]; then
  args+=(-kubernetes-version "${kube_version#v}")
fi

# Schemas of Kubernetes kinds, then of common CRDs.
args+=(
  -schema-location default
  -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json'
)

# Only the sliced manifests: static/ holds files that are not resources.
shopt -s nullglob
manifests=("$MYKS_RENDERED_APP_DIR"/*.yaml)
if ((${#manifests[@]} == 0)); then
  exit 0
fi
kubeconform "${args[@]}" "$@" "${manifests[@]}"
```

```shell
chmod +x plugins/validate
myks render && myks validate
```

The script reads the Kubernetes version from `helm.kubeVersion`, so each
environment is validated against the version its cluster runs. Kinds without a
published schema fail validation, `CustomResourceDefinition` among them; skip
them with `myks validate -- -skip CustomResourceDefinition`, or pass
`-ignore-missing-schemas`. The script needs `kubeconform` and `yq` on `PATH`.

## Example: deploy with kapp

`plugins/kapp`:

```bash
#!/usr/bin/env bash
kapp deploy --yes --wait -a "$MYKS_APP" -f "$MYKS_RENDERED_APP_DIR" "$@"
```

```shell
chmod +x plugins/kapp
myks kapp prod/eu frontend
```
