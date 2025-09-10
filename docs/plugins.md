# Plugins

With plugins the rendered output of `myks` can be acted on with additional
tools. To name but a few use cases where this is useful:

- You are using `myks` without a GitOps CD tool like ArgoCD. With a `myks`
  plugin you can leverage a local deployment tool like `kubectl` or `kapp` to
  deploy your rendered K8s manifests to locally accessible clusters.
- You are using `myks` without a secret management system like HashiCorp Vault.
  With a `myks` plugin you can decrypt your locally held secrets before applying
  them and re-encrypt them afterward.
- You would like to validate your rendered K8s manifests with a tool like
  `kubeconform` or `conftest`.

## Installation

Myks recognizes all executables on your `PATH` that are prefixed with `myks-` as
plugins.

Additionally, you can put executables of any name in a local `plugins` directory
in the root of your project. Further plugin source directories can be configured
in your `.myks.yaml`:

```yaml
# Load all binaries from the following folders as myks plugins.
plugin-sources:
  - ./plugins
  - ./custom-tools
  - /usr/local/share/myks-plugins
```

The default plugin directory is `./plugins` relative to your project root. You
can specify multiple directories, and myks will search them all for executable
files to load as plugins.

## Plugin execution logic and environment variables

Like myks' render logic, a plugin is executed for every environment and
application provided during the invocation of `myks` or for whatever environment
and application is detected by the Smart Mode.

For every plugin execution, the following environment variables are injected to
enable your plugin to act on the rendered YAMLs of the current environment and
application:

| Variable              | Description                                                        |
| --------------------- | ------------------------------------------------------------------ |
| MYKS_ENV              | ID of currently selected environment                               |
| MYKS_APP              | Name of currently selected application                             |
| MYKS_ENV_DIR          | Path to config directory of currently selected environment         |
| MYKS_APP_PROTOTYPE    | Path to prototype directory of currently selected application      |
| MYKS_RENDERED_APP_DIR | Path to render directory of currently selected application         |
| MYKS_DATA_VALUES      | Yaml with the configuration data values of the current application |

## Example: `myks-kapp` plugin

The following is an example of a `myks` plugin that uses `kapp` to deploy your
rendered K8s manifests to a locally accessible cluster.

1. Create a file named `kapp` in a `plugins` directory sitting at the root of
   your GitOps repository with the following content:

   ```bash
     #! /usr/bin/env bash
     kapp --yes deploy --wait -a "$MYKS_APP" -f "$MYKS_RENDERED_APP_DIR"
   ```

1. Make the file executable:

   ```bash
   chmod +x plugins/kapp
   ```

1. Test whether myks successfully picks up your plugin. It should appear in the
   output of `myks help`:

   ```bash
   myks help
   ```

1. Run the plugin:

   ```bash
   myks kapp my-env my-app
   ```
