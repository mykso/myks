# Ytt Libraries

Myks provides flexible ways to use
[ytt libraries](https://carvel.dev/ytt/docs/latest/lang-ref-ytt-library/) in
your applications and prototypes.

## Environment Data Library

Environment Data Library allows you to access an environment's known data values
in an application's data values files as a ytt library. Normally, ytt doesn't
provide a way to reference other data values, because all data values are loaded
simultaneously.

During processing of an environment, myks internally produces final data values
for the environment from collected data values of the parent environments. For
every ytt invocation scoped to an application, myks provides a `@myks` library
with a `data.lib.yaml` module which exports a struct `env_data` containing all
final data values of the environment. Here's how it can be used:

```yaml
#@ load("@myks:data.lib.yaml", "env_data")

#@data/values
---
application:
  name: #@ "MyApp-" + env_data.environment.region
```

The `@myks` library at the moment is not added when the ytt invocation is not
scoped to an application, e.g. when rendering environment-level overlays. In
such cases, you can directly reference the same data values by loading them from
the standard `@ytt:data` library.

## Application and Prototype Libraries

In addition to the global environment data library, myks automatically includes
`lib` directories found in application and prototype paths. This allows you to
define and share reusable ytt functions or data across your project.

### Directory Structure

Myks looks for a `lib` directory in the following locations:

1. **Application Directories**: `envs/**/_apps/<app>/lib`
   - Libraries specific to an application in a specific environment (or any
     parent environment).
2. **Prototype Directories**: `prototypes/<prototype>/lib`
   - Libraries shared by all applications using this prototype.

### Usage

Any `.lib.yaml` or `.star` files placed in these `lib` directories are available
to be loaded in your ytt templates, overlays, and data values.

For example, if you have:

`prototypes/my-app/lib/helpers.lib.yaml`:

```yaml
#@ def name_suffix(suffix):
#@   return "-"+suffix
#@ end
```

You can use it in your application's `ytt` files:

```yaml
#@ load("helpers.lib.yaml", "name_suffix")

apiVersion: v1
kind: ConfigMap
metadata:
  name: #@ "myapp" + name_suffix("prod")
```

### File Collection Order

Myks collects library files in a specific order. This is important if you have
data values files in these directories (which are order-sensitive) or if relying
on specific file loading behavior.

The collection order is:

1. **Global Environment Data** (`.myks/envs/...`)
2. **Application Libraries** (collected from the most specific environment to
   the root environment)
3. **Prototype Libraries**
4. **Environment Data Files**
