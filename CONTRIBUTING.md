# Contributing to myks

Bug reports, documentation fixes and features are welcome. For anything larger
than a small fix, open an issue or a
[discussion](https://github.com/mykso/myks/discussions) first, so that we can
agree on the approach before you write the code.

## Development environment

The Nix flake provides every tool:

```shell
nix develop
```

Without Nix, install:

- [Go](https://go.dev/) (the version in `go.mod`);
- [Helm](https://helm.sh/), for the tests that render charts;
- [goreleaser](https://goreleaser.com/), for `task go:build`;
- optionally [task](https://taskfile.dev/),
  [lefthook](https://github.com/evilmartians/lefthook),
  [gofumpt](https://github.com/mvdan/gofumpt),
  [goimports-reviser](https://github.com/incu6us/goimports-reviser),
  [golangci-lint](https://golangci-lint.run/) and
  [commitlint](https://commitlint.js.org/).

Run `lefthook install` once to enable the Git hooks: `go vet`, `gofumpt` and
`golangci-lint` before a commit, commitlint on the message.

## Build, test, lint

```shell
task go:build   # bin/myks, built with goreleaser
task go:test    # builds, then runs go test -race with bin/ on PATH
task go:lint    # golangci-lint
task go:fmt     # goimports-reviser and gofumpt
```

Without task:

```shell
go build -o bin/myks .
PATH="$PWD/bin:$PATH" go test -failfast -race ./...
```

The tests call the `myks` binary for the embedded tools, so it must be on
`PATH`. The example projects in [`examples/`](examples/README.md) double as
integration test fixtures: their `rendered/` directories are the expected
output. Do not edit them by hand; re-render them when a change is intended.

[AGENTS.md](AGENTS.md) describes the code layout and conventions in more detail.

## Pull requests

- Pull request titles follow
  [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`,
  `docs:`, …). Release notes and version numbers are generated from them.
- `feat`, `fix` and `perf` pull requests need a `## Release note` section in the
  description (see the
  [pull request template](.github/pull_request_template.md)): what a user
  notices, or "none". The release pull request collects these notes as the
  highlights of the release.
- CI runs lint and tests on every pull request; both must pass.
- Update the documentation in [`docs/src/content/docs/`](docs/src/content/docs/)
  together with the behavior it describes. Pages under `reference/cli/` are
  generated: rebuild myks and run
  `npx help2md --bin ../bin/myks --out src/content/docs/reference/cli` from
  `docs/` after changing a command's help.

## Releases

[release-please](https://github.com/googleapis/release-please) keeps a release
pull request open with the changes since the last release. Merging it tags the
release; goreleaser then builds the binaries, packages and container images, and
publishes them to GitHub Releases, the container registry and the Homebrew tap.
The release pull request additionally requires a successful goreleaser build.
