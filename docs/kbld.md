# Kbld Integration

[kbld](https://carvel.dev/kbld/) is a tool that resolves image tags to immutable
digests, improving security and reproducibility of Kubernetes deployments. Myks
integrates with kbld to automatically resolve image references in your rendered
manifests.

> [!NOTE]  
> This feature is currently experimental and may change in future releases. The
> configuration schema and behavior may evolve. It is disabled by default and
> can be enabled via data-values files.

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
  #! Image reference overrides (see below for details).
  overrides: []
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

## Image Reference Overrides

The `kbld.overrides` configuration allows you to rewrite image references before
they are resolved to digests. This is useful for scenarios like:

- Redirecting images from Docker Hub to a private registry
- Changing repository paths for compliance or security requirements
- Applying organization-wide image policies

### How Overrides Work

For each image reference detected by kbld, myks searches for the first matching
override and applies it. Each override consists of:

- **match**: Regular expressions applied to parts of the image reference
  (registry, repository, tag)
- **replace**: Replacement values that can reference capture groups from the
  match expressions

### Configuration Rules

- Match values are regular expressions that are **implicitly anchored** to the
  start and end of the string (no need to add `^` and `$`)
- If a part is not specified in the match, it matches anything
- If a part is not specified in the replace, the original value is kept
- Replace values can reference capture groups from match expressions using `$1`,
  `$2`, etc.
- See [Go's regexp documentation](https://pkg.go.dev/regexp#Regexp.ReplaceAllString)
  for syntax details

### Docker Hub Reference Handling

Myks normalizes Docker Hub references before matching:

- Images without a registry are normalized to `index.docker.io`
- `docker.io` registry references are normalized to `index.docker.io`
- Docker Hub repositories without a prefix are in the `library/` namespace

Examples:
- `nginx` → `registry=index.docker.io`, `repository=library/nginx`, `tag=latest`
- `docker.io/nginx:latest` → `registry=index.docker.io`,
  `repository=library/nginx`, `tag=latest`

### Examples

#### Redirect All Images to a Private Registry

```yaml
kbld:
  enabled: true
  overrides:
    - match:
        registry: index\.docker\.io
      replace:
        registry: my-private-registry.local
```

This changes all Docker Hub images to use your private registry while keeping
the repository path and tag unchanged.

#### Rewrite Bitnami Images to a Legacy Repository

```yaml
kbld:
  enabled: true
  overrides:
    - match:
        repository: bitnami/(.+)
      replace:
        registry: my-private-registry.local
        repository: bitnamilegacy/$1
```

This override:
- Matches any Bitnami image (e.g., `bitnami/postgresql`)
- Changes the registry to your private registry
- Rewrites the repository path from `bitnami/*` to `bitnamilegacy/*`
- Keeps the original tag

#### Multiple Overrides

```yaml
kbld:
  enabled: true
  overrides:
    # First, redirect Docker Hub images
    - match:
        registry: index\.docker\.io
      replace:
        registry: my-registry.local
    # Then, handle special case for Bitnami images
    - match:
        registry: my-registry\.local
        repository: bitnami/(.+)
      replace:
        repository: legacy/$1
```

Overrides are processed in order, and only the first match is applied. Design
your overrides carefully when using multiple rules.

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
