# Environment Data Library

Environment Data Library allows you to access an environment's data values in an
application's data values files as a ytt library. Normally, ytt doesn't provide
a way to reference other data values, because all data values are loaded
simultaneously.

During processing of an environment, myks internally produces final data values
for the environment from collected data values of the parent environments. For
every ytt invocation scoped to an application, myks adds an `env-data.lib.yaml`
file which can be loaded as a usual ytt library.

> **Note:** `env-data.lib.yaml` is a reserved file name used by myks. Do not
> create files with this name in your repository.

```yaml
#@ load("env-data.lib.yaml", "env_data")

#@data/values
---
application:
  name: #@ "MyApp-" + env_data.environment.region
```

In this example, the `env_data` library is loaded from the `env-data.lib.yaml`
file, which contains the final calculated data values of the application's
environment.

This library is not added when the ytt invocation is not scoped to an
application, e.g. when rendering environment-level overlays. In such cases, you
can directly reference the same data values by loading them from the standard
`@ytt:data` library.
