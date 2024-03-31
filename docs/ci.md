# CI

We develop on feature branches and create pull requests to merge them into the
default branch. The CI pipeline is triggered on every push to a pull request.

If there are changes on the default branch that are not yet released, the
release-please action will create a new release PR. After the release PR is
merged, a new Github release will be created and the version will be bumped in
the default branch.

## PR Checks

The following checks are required to pass before a PR can be merged:

- lint
- test
- PR title follows conventional commits format

## Release PR Checks

Before a release PR can be merged, in addition to the PR checks, the following
checks are required to pass:

- goreleser build

## Releasing

After a PR is merged into the default branch,
