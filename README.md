# Myks

**M**anage (**my**) **y**aml for **K**ubernetes **s**imply. Or something like that.

**Myks** helps to maintain configuration of many applications for multiple Kubernetes clusters.

## Development

### Prerequisites

For building and contributing:

- [Go](https://golang.org/) 1.20+
- [goreleaser](https://goreleaser.com/) 1.18+
- optional:
  - [task](https://taskfile.dev/) 3.27+
  - [lefthook](https://github.com/evilmartians/lefthook) 1.4+
  - [gofumpt](https://github.com/mvdan/gofumpt) 0.5+
  - [golangci-lint](https://golangci-lint.run/) 1.53+
  - [commitlint](https://commitlint.js.org/#/) 17.6+

For running:

- [ytt](https://get-ytt.io/) 0.44+
- [vendir](https://carvel.dev/vendir/) 0.34+
- [helm](https://helm.sh/) 3.12+

### Build

```console
$ task go:build
$ # or, if task or goreleaser aren't installed, just
$ go build -o myks ./cmd/myks
```

### Test

```console
$ # Switch to an empty directory
$ cd $(mktemp -d)
$ # Initialize a new project
$ myks init
$ # Optionally, check the generated files
$ find
$ # Sync and render everything
$ myks all envs --log-level debug
```

### Run

#### Running `sync` against protected repositories and registries

Vendir uses `secret` resources to authenticate against protected repositories. These are references by the `vendir.yaml` with the `secretRef` key. 

Myks dynamically creates these secrets based on environment variables prefixed with `VENDIR_SECRET_`.
For example, if you reference a secret named "mycreds" in your `vendir.yaml, you need to define the environment variables VENDIR_SECRET_MYCREDS_USERNAME` and `VENDIR_SECRET_MYCREDS_PASSWORD`. The secrets are cleaned up automatically after the sync is complete.
