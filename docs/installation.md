# Installation

Myks is a single binary with [ytt], [vendir] and [kbld] built in. Two external
tools are needed at runtime:

- [helm] 3.12 or newer, only if your applications render Helm charts;
- [git], for [Smart Mode](smart-mode.md) and for the Argo CD defaults that are
  read from the repository (URL and branch).

Myks does not track compatibility with specific Helm versions. Recent versions
work.

## Homebrew

```shell
brew install --cask mykso/tap/myks
```

## Nix

The package is available in Nixpkgs as
[`myks`](https://search.nixos.org/packages?channel=unstable&show=myks&query=myks):

```shell
nix-shell -p myks kubernetes-helm git
```

The Nixpkgs version can lag behind the latest release. To run the latest
version, use the flake of this repository:

```shell
nix shell 'github:mykso/myks/main#myks' 'nixpkgs#kubernetes-helm' 'nixpkgs#git'
```

## AUR

On Arch-based distributions, install
[`myks-bin`](https://aur.archlinux.org/packages/myks-bin/):

```shell
yay -S myks-bin
```

## APK, DEB, RPM

The [latest release page](https://github.com/mykso/myks/releases/latest) lists
packages for Alpine, Debian-based and RPM-based distributions.

## Docker

Images are published to the
[GitHub container registry](https://github.com/mykso/myks/pkgs/container/myks).
They include `helm` and `git`, use `/app` as the working directory and run
`myks` as the entry point:

```shell
docker run --rm -v "$PWD:/app" ghcr.io/mykso/myks:latest render
```

## GitHub Actions

[mykso/setup-action](https://github.com/mykso/setup-action) installs a pinned
version of myks in a workflow. See [GitOps and CI](gitops-ci.md) for complete
workflows.

## Binary archives

Download an archive for your platform from the
[releases page](https://github.com/mykso/myks/releases) and put the `myks`
binary on your `PATH`.

## From source

`go install github.com/mykso/myks@latest` does not work, because `go.mod`
replaces two dependencies with forks. Build from a checkout instead:

```shell
git clone https://github.com/mykso/myks.git
cd myks
go build -o myks .
```

## Shell completion

```shell
# bash
source <(myks completion bash)
# zsh
myks completion zsh > "${fpath[1]}/_myks"
# fish
myks completion fish | source
```

## Pinning the version for a project

Set `min-version` in the project's `.myks.yaml` so that older binaries refuse to
run it (see [Configuration](configuration.md#min-version)):

```yaml
min-version: 'v5.16.0'
```

[git]: https://git-scm.com/
[helm]: https://helm.sh/
[kbld]: https://carvel.dev/kbld/
[vendir]: https://carvel.dev/vendir/
[ytt]: https://carvel.dev/ytt/
