# Kbld Integration

[kbld](https://carvel.dev/kbld/) is a tool that resolves image tags to immutable
digests, improving security and reproducibility of Kubernetes deployments. Myks
integrates with kbld to automatically resolve image references in your rendered
manifests.

> [!NOTE] This feature is currently experimental and may change in future
> releases. The configuration schema and behavior may evolve. It is disabled by
> default and can be enabled via data-values files.

## Configuration

To enable kbld integration, set `.kbld.enabled` to `true` in a data-values file
at the root, prototype, environment, or application level.

```yaml
kbld:
  enabled: false
  #! Annotate resources with images annotation.
  #! See --images-annotation flag in kbld docs for details.
  imagesAnnotation: true
  #! Cache resolved references in the local myks cache.
  cache: true
```

### How It Works

When enabled, myks will:

1. Render the application manifests using ytt/helm
2. Run kbld to resolve all image tags to digests
3. Write the updated manifests to the render directory

For example, an image reference like `nginx:1.21` would be resolved to something
like `nginx@sha256:abc123...`.

## Advanced Configuration

Kbld can be further configured using its own configuration documents. Include a
kbld configuration in your ytt templates, and it will be processed by kbld and
removed from the final rendered manifests.

Example kbld configuration:

```yaml
---
apiVersion: kbld.k14s.io/v1alpha1
kind: Config
minimumRequiredVersion: 0.15.0
searchRules:
  - keyMatcher:
      name: data.yml
    updateStrategy:
      yaml:
        searchRules:
          - keyMatcher:
              name: image
```

See the
[kbld configuration documentation](https://carvel.dev/kbld/docs/develop/config/)
for complete details.

## Caching

When `kbld.cache` is set to `true`, kbld will create a `kbld-lock.yaml` file in
the application's service directory (typically under `.myks/envs/`). This file
caches resolved image references to speed up subsequent renders.

The lock file:

- Contains resolved image digests and metadata
- Is automatically created on first run
- Is updated when new or changed images are detected
- Should typically be committed to version control or cached in CI for
  reproducible builds

If the lock file is missing, kbld will resolve all images and recreate it.
