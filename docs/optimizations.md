# Myks Optimizations

This page lists myks' features that help to optimize performance and resource
usage. Some of those features are enabled by default, while others require
additional configuration. There are features that improve rendering speed, but
the most useful are those that reduce syncing activities.

## Parallelism

Myks can process multiple applications in parallel. It is applied to both
syncing and rendering operations. The number of parallel operations can be
configured with the `--async` flag, with the `MYKS_ASYNC` environment variable,
or with the `async` configuration key. It is set to `0` by default, which means
that there is no limit.

## Selective Processing

### Smart Mode

Myks can detect what applications and environments to process based on the
changes in the source files. This allows to reduce the number of applications
and environments to process, which in turn reduces the time needed for syncing
and rendering. See the [Smart Mode](/docs/smart-mode.md) page for more details.

### Manual Selection

Sometimes, the Smart Mode may not be able to detect the changes correctly. In
this case, you can manually specify the applications and environments to
process. For example:

```shell
myks render foo-env,bar-env baz-app
```

This command will sync and render the `baz-app` application in the `foo-env` and
`bar-env` environments only.

See `myks help` for more details.

## Skip Sync

### Vendir's `lazy` mode

Vendir has a `lazy` mode that skips syncing of the sources that were synced
earlier and haven't changed their vendir configuration. See the
[`vendir.yml` Reference](https://carvel.dev/vendir/docs/v0.40.x/vendir-spec/)
for more details.

It is advised to enable the `lazy` mode in the `vendir.yml` configuration file
if the corresponding sources are defined using immutable[^1] references (e.g.,
commit hashes or image digests). For example:

```yaml
apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
  - path: charts/cert-manager
    contents:
      - path: .
        lazy: true # <--
        helmChart:
          name: cert-manager
          version: v1.14.5
          repository:
            url: https://charts.jetstack.io
```

Here, we trust maintainers of the cert-manager chart to not move the `v1.14.5`
tag and ask vendir to skip syncing this source if it was downloaded earlier.

[^1]:
    Some references aren't strictly immutable, but can be treated as such in
    practice. For example, a git tag can be moved, but a user can decide to
    treat it as immutable.

### Centralized Cache

Using the vendir's `lazy` mode is a good way to avoid unnecessary syncing of
unchanged sources. However, it doesn't help when the same source is used in
multiple applications. In this case, the source is synced multiple times for
multiple applications.

To overcome this limitation, myks implements a centralized cache and ensures
that only one sync is performed for each source. The same source is made
available to multiple applications by symlink creation.

Cache entries are stored in the `.myks/vendir-cache` directory and identified by
a hash of the corresponding `contents` section in the `vendir.yml` file. The
cache is invalidated when the vendir configuration changes.

Cache entries that are not used by any application can be removed with the
`cleanup` command:

```shell
myks cleanup --cache
```

### Speed Up CI/CD Pipelines

Persisting the cache between CI jobs can significantly reduce the time needed
for syncing. Generally, there are two ways to achieve this:

- utilize a shared cache between CI jobs that is provided by your CI platform.
- check-in the cache directory to the repository by adjusting the gitignore
  rules as follows:

  ```diff
  -.myks
  +.myks/*
  +!.myks/vendir-cache
  ```

  Note, that this approach may introduce a lot of changes and increase the
  repository size. If you commit changes after syncing (in CI, as well as
  locally), make sure to run the `cleanup` command before. This will reduce
  diffs and make them cleaner.
