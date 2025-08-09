# Smart Mode

Smart Mode looks at the changes you made and figures out which environments and
applications need syncing or rendering.

It is activated by default.

## How it works

The Smart Mode tries to be as efficient as possible by only processing the parts
of your configuration that are affected by the changes you made. This is done by
comparing the last state of your configuration with the current state. The
decisions are based purely on names of files and directories, not on their
content. Therefore, it is not always possible to avoid unnecessary processing.

There are several scenarios in which myks will decide to process different parts
of your configuration:

- nothing
- one or multiple applications
- one or multiple environments
- all environments and all applications

The processing scope goes from the smallest (nothing) to the largest
(everything). A broader scope always includes the narrower ones. Than means, if
a particular environment is selected for processing, all applications of that
environment will be processed, no matter if they have changed or not.

You can see the scope of processing using the `--smart-mode.only-print` flag:

```console
$ myks render --smart-mode.only-print

Smart Mode detected:
→ envs/alpha
    traefik
```

### Nothing to process

If there are no changes that would have an impact on the rendered output of your
workloads (e.g. only `Dockerfile` is modified), myks will exit immediately,
before any syncing or rendering happens.

### Processing of one or multiple applications

An application of a specific environment gets processed in any of the following
cases:

- The `app-data.ytt.yaml` of that application has changed, for example:
  - `envs/env-1/_apps/app-1/app-data.ytt.yaml`
- Any files of the known plugins have changed, for example:
  - `.../app-1/ytt/...`
  - `.../app-1/helm/...`
- The prototype of that application has changed, for example:
  - `prototypes/app-1/vendir/...`

> [!NOTE] In the latter case, when a prototype has changed, all applications
> that use this prototype are selected for processing.

### Processing all applications of one or multiple environments

All applications of an environment are processed in any of the following cases:

- The `env-data.ytt.yaml` of that environment has changed, for example:
  - `envs/env-group/env-1/env-data.ytt.yaml`
- The `env-data.ytt.yaml` of any of the parent environments has changed, for
  example:
  - `envs/env-group/env-data.ytt.yaml`
- Any files of the known environment plugins have changed, for example:
  - `.../env-1/_env/ytt/...`
  - `.../env-1/_env/argocd/...`

> [!NOTE] Changing the upper-level environment (e.g. `/envs/env-data.ytt.yaml`)
> will naturally promote the scope of processing to all environments and
> applications, as all environments depend on the upper-level one.

### Processing all environments and all applications

A complete rendering of all environments and all applications is currently
required only when the common lib directory has changed, for example:

- `/lib/common.lib.star`
