# Myks

**M**anage (**my**) **y**aml for **K**ubernetes **s**imply. Or something like
that.

**Myks** is a tool and a framework for managing the configuration of
applications for multiple Kubernetes clusters. It helps to reuse, mutate, and
share the configuration between applications and clusters.

Basically, myks downloads sources and renders them into ready-to-use Kubernetes
manifests.

Join our [Slack channel](https://kubernetes.slack.com/archives/C06BVDBHZC2)!

## Features

- create and reuse application "prototypes" combining helm charts, ytt templates
  and overlays, and plain YAML files;
- download and cache third-party configurations from various sources (anything
  that is supported by [vendir]: helm charts, git repositories, GitHub releases,
  etc.);
- maintain a single source of truth for all clusters' manifests;
- render manifests, validate and automatically inspect them before applying;
- [smart detection](/docs/smart-mode.md) of what to render based on what was
  changed;
- integrate with ArgoCD;
- apply changes to all applications in all clusters at once or to a specific
  subset;
- [plugins](/docs/plugins.md) support for extending myks with custom tools.

## How does it work?

Myks consumes a set of templates and values and renders them into a set of
Kubernetes manifests. It is built on top of [vendir] for retrieving sources and
on top of [ytt] for configuration.

### Hands on

Here's a quick example:

```shell
# Switch to an empty directory
cd "$(mktemp -d)"

# Initialize a repository
git init

# Make an initial commit
git commit --allow-empty -m "Initial commit"

# Initialize a new project with example configuration
myks init

# Optionally, check the generated files
find

# Sync and render everything
myks render

# Check the rendered manifests
find rendered
```

## Installation

Depending on the installation method and on the desired features, you may need
to install some of the tools manually:

- [helm] is only needed for rendering Helm charts
- [ytt] and [vendir] are now built into myks, no need to install separately.

At the moment, we do not track compatibility between versions of these tools and
myks. Fairly recent versions should work fine.

### AUR

On Arch-based distros, you can install
[`myks-bin`](https://aur.archlinux.org/packages/myks-bin/) from AUR:

```shell
yay -S myks-bin
```

### APK, DEB, RPM

See the [latest release page](https://github.com/mykso/myks/releases/latest) for
the full list of packages.

### Docker

See the
[container registry page](https://github.com/mykso/myks/pkgs/container/myks) for
the list of available images. The image includes the latest versions of `helm`.

```shell
docker pull ghcr.io/mykso/myks:latest
```

### Homebrew

```
brew tap mykso/tap
brew install myks
```

### Nix

The package is available in the Nixpkgs repository under the name
[`myks`](https://search.nixos.org/packages?channel=unstable&show=myks&from=0&size=50&sort=relevance&type=packages&query=myks).

```
nix-shell -p myks kubernetes-helm git
```

> [!NOTE]  
> The version in Nixpkgs is falling behind the latest release. If you need the
> latest version, use the flake.
>
> ```shell
> nix shell 'github:mykso/myks/main#myks' 'nixpkgs#helm' 'nixpkgs#git'
> ```

### Download manually

Download an archive for your OS from the
[releases page](https://github.com/mykso/myks/releases) and unpack it.

### Build from source

Get the source code and build the binary:

```shell
git clone https://github.com/mykso/myks.git
# OR
curl -sL https://github.com/mykso/myks/archive/refs/heads/main.tar.gz | tar xz

cd myks-main
go build -o myks main.go
```

## Usage

To become useful, myks needs to be run in a project with a particular directory
structure and some basic configuration in place. A new project can be
initialized with `myks init` (see [an example](#hands-on)).

Myks has two main stages of operation: sync and render. During the sync stage,
myks downloads and caches external sources. Final kubernetes manifests are
rendered from the retrieved and local sources during the render stage.

The `render` command handles both stages and accepts optional flags to control
behavior:

- By default, it runs both sync and render stages sequentially
- Use `--sync` to only sync external sources
- Use `--render` to only render manifests

The `render` command accepts two optional arguments (comma-separated lists or
`ALL`): environments and applications to process. When no arguments are
provided, myks will use the [Smart Mode](/docs/smart-mode.md) to detect what to
process.

> [!TIP]  
> Check the [optimizations](/docs/optimizations.md) page to get most of myks.

### Examples

A few example setups are available in the [examples](/examples) directory.

And here are some real-world examples:

- [zebradil/myks-homelab](https://github.com/zebradil/myks-homelab): single
  cluster setup with [ArgoCD] for deployment and [sops] for secrets management;
- [kbudde/lab](https://github.com/kbudde/lab): single cluster setup with [kapp]
  for deployment and [sops] for secrets management;

### Running `sync` against protected repositories and registries

Vendir uses `secret` resources to authenticate against protected repositories.
These are references by the `vendir.yaml` with the `secretRef` key.

Myks dynamically creates these secrets based on environment variables prefixed
with `VENDIR_SECRET_`. For example, if you reference a secret named "mycreds" in
your `vendir.yaml`, you need to define the environment variables
`VENDIR_SECRET_MYCREDS_USERNAME` and `VENDIR_SECRET_MYCREDS_PASSWORD`. The
secrets are cleaned up automatically after the sync is complete.

## Development

### Prerequisites

For building and contributing:

- [Go](https://golang.org/) 1.22+
- [goreleaser](https://goreleaser.com/) 1.18+
- optional:
  - [task](https://taskfile.dev/) 3.27+
  - [lefthook](https://github.com/evilmartians/lefthook) 1.4+
  - [gofumpt](https://github.com/mvdan/gofumpt) 0.5+
  - [golangci-lint](https://golangci-lint.run/) 1.53+
  - [commitlint](https://commitlint.js.org/#/) 17.6+

For running:

- [helm] 3.12+

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
$ # Initialize a repository
$ git init
$ # Make an initial commit
$ git commit --allow-empty -m "Initial commit"
$ # Initialize a new myks project
$ myks init
$ # Optionally, check the generated files
$ find
$ # Sync and render everything
$ myks render envs --log-level debug
```

## Motivation

The original idea grew out of the need to maintain applications in a constantly
growing zoo of Kubernetes clusters in a controlled and consistent way.

Here are some of the requirements we had:

- to be able to create and maintain configurations of multiple applications for
  multiple clusters;
- to provide compatibility tools for different Kubernetes flavors (e.g. k3s,
  Redshift, AKS) and versions;
- to be able to consume upstream application configurations in various formats
  (e.g. Helm, kustomize, plain YAML);
- to have automatic updates and version management;
- to provide a single source of truth for the configuration.

[//]: # 'Links'
[ArgoCD]: https://argoproj.github.io/cd/
[helm]: https://helm.sh/
[sops]: https://github.com/getsops/sops
[vendir]: https://carvel.dev/vendir/
[ytt]: https://carvel.dev/ytt/
