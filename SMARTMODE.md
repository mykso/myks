# Smart Mode

Smart Mode looks at the changes you made and figures out which environments and applications need re-rendering.

It is activated by default.

## How it works

### No changes relative to rendering

If you make changes that have no impact on the rendered output of your workloads, e.g. modify `Dockerfile`, myks
will exit immediately, before any syncing or rendering happens.

### Re-rendering of an application

An application of a specific environment get re-rendered when:
- The `app-data.ytt.yaml` of that application has changed, e.g. `envs/env-1/_apps/app-1/app-data.ytt.yaml`
- The prototype application has changed, e.g. `prototypes/app-1/helm/app-1.yaml`, in which case all environments that use is re-render that application.

### Re-rendering of an environment

All applications of an environment get re-rendered when:
- The `env-data.ytt.yaml` of that environment has changed.
- The `env-data.ytt.yaml` of a parent environment of that environment has changed.
- **Edge case:** If you have made changes to an application in env-1, but at the same time have modified the `env-data.ytt.yaml` of env-2, smart-mode will re-render all applications of env-1 AND env-2, even though this is not strictly required for env-1.

### Complete rendering

A complete rendering of all environments and all applications is required when:
- A file in the common lib has changed, e.g. `/lib/common.lib.star`
- A file in the global ytt folder has changed, e.g. `/envs/_env/ytt/annotate_crds.yaml`
- The base `env-data.ytt.yaml` has changed: `/envs/env-data.ytt.yaml`


