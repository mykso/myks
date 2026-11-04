# GitOps and CI

Myks writes its output to `rendered/`, and your deployment tool applies what is
committed there. Two things keep that safe: every change to the inputs arrives
together with its rendered output, and nothing reaches the default branch with
stale output. This page shows CI workflows for both, with GitHub Actions
examples that translate to any CI system.

## The workflow

1. A contributor changes inputs in a branch and runs `myks render`, or lets CI
   do it.
2. CI renders the pull request, validates the result, and either commits the
   output or fails when the committed output is stale.
3. Reviewers read the `rendered/` diff.
4. After the merge, a check on the default branch catches output that is still
   stale (for example after a merge race), and the deployment tool syncs.

## Rendering only what changed

[Smart Mode](smart-mode.md) selects what to render from the files that differ
from a base revision. It compares that revision with the working tree, so in a
pull request use the merge base, not the tip of the target branch. Otherwise
commits that landed on the target branch after the branch was created count as
changes too:

```shell
base=$(git merge-base origin/main HEAD)
myks render --smart-mode.base-revision="$base"
```

The checkout needs enough history for the merge base: fetch the full history
(`fetch-depth: 0`), or at least the target branch.

Preview the selection without rendering, for example to post it as a comment:

```shell
myks render --smart-mode.base-revision="$base" --smart-mode.only-print=json
```

## Pull requests: commit the output

This workflow renders what the pull request changed and pushes the result to the
pull request branch, so contributors do not need myks installed. It works for
branches of the same repository; pushing to forks needs a different setup.

```yaml
# .github/workflows/render.yaml
name: Render manifests
on:
  pull_request:

permissions:
  contents: write

jobs:
  render:
    runs-on: ubuntu-latest
    env:
      # renovate: datasource=github-releases depName=mykso/myks
      MYKS_VERSION: v5.16.0
    steps:
      - uses: actions/checkout@v7
        with:
          ref: ${{ github.head_ref }}
          fetch-depth: 0

      - uses: mykso/setup-action@v3.0.16
        with:
          myks: ${{ env.MYKS_VERSION }}
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Restore the source cache
        uses: actions/cache@v6
        with:
          path: .myks/vendir-cache
          key: myks-vendir-${{ github.run_id }}
          restore-keys: myks-vendir-

      - name: Render
        run: |
          base=$(git merge-base "origin/${{ github.base_ref }}" HEAD)
          myks render --smart-mode.base-revision="$base"
          myks cleanup --manifests

      - name: Commit the rendered output
        run: |
          git add rendered
          git diff --cached --quiet && exit 0
          git config user.name 'github-actions[bot]'
          git config user.email 'github-actions[bot]@users.noreply.github.com'
          git commit -m 'chore: render manifests'
          git push
```

GitHub-hosted Ubuntu runners include Helm. On other runners, install it before
rendering.

Commits pushed with the default `GITHUB_TOKEN` do not trigger new workflow runs.
If other required checks must run on the rendered commit, push with a GitHub App
token or a deploy key instead.

> [!TIP]  
> A Smart Mode render deletes the output of applications removed from an
> environment, but not of environments that no longer exist.
> `myks cleanup --manifests` removes both.

## Pull requests: verify the output

If contributors render locally, CI only checks that they did. Replace the last
step of the workflow above with:

```yaml
- name: Check that rendered/ is up to date
  run: |
    git add --intent-to-add rendered
    git diff --exit-code --stat rendered
```

`--intent-to-add` makes new files show up in the diff, so a forgotten new
application fails the check too.

## Validating rendered manifests

Rendering succeeds long before the result is deployable. A chart can emit a
manifest the API server rejects, or a ytt directive can end up in the output
verbatim. Run validators on the rendered output as a [plugin](plugins.md), with
the same selection as the render:

```yaml
- name: Validate
  run: |
    base=$(git merge-base "origin/${{ github.base_ref }}" HEAD)
    myks validate --smart-mode.base-revision="$base"
```

The [plugins page](plugins.md) has a `validate` plugin built on kubeconform. The
same mechanism runs `promtool check rules` on Prometheus rules, `conftest`
policies, or any other tool that takes file paths.

`myks inspect apps --redundant` exits non-zero when a values file sets a value
to what it already inherits. Run it in CI to keep overrides minimal (see
[Inspecting and debugging](inspect.md)).

## The default branch: catch drift

Even with checks on pull requests, the default branch can end up with stale
output: two pull requests that touch the same application merged one after the
other, a check that was skipped, a direct push. Render every push to the default
branch against the previous commit, and fail when the output changes:

```yaml
# .github/workflows/drift.yaml
name: Check rendered manifests
on:
  push:
    branches: [main]

jobs:
  drift:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - uses: mykso/setup-action@v3.0.16
        with:
          myks: v5.16.0
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Render
        run: myks render --smart-mode.base-revision=${{ github.event.before }}

      - name: Fail on drift
        run: |
          git add --intent-to-add rendered
          git diff --exit-code --stat rendered
```

Instead of failing, the job can open a pull request with the re-rendered output
(for example with `peter-evans/create-pull-request`) and assign it to the author
of the commit. Keep the base revision of the first detection while that pull
request is open: rendering later pushes against their own previous commits makes
Smart Mode skip the applications that drifted.

A scheduled full render (`myks render ALL ALL`) catches the remaining case:
sources with moving references, such as Git branches synced with `lazy: false`,
change without any commit in your repository.

## Private sources in CI

Pass credentials for protected chart repositories and Git servers as
`VENDIR_SECRET_<NAME>_USERNAME` and `VENDIR_SECRET_<NAME>_PASSWORD` environment
variables from CI secrets (see
[Sources](sources.md#private-repositories-and-registries)):

```yaml
- name: Render
  env:
    VENDIR_SECRET_CHARTS_USERNAME: ${{ secrets.CHARTS_USERNAME }}
    VENDIR_SECRET_CHARTS_PASSWORD: ${{ secrets.CHARTS_PASSWORD }}
  run: myks render --smart-mode.base-revision="$base"
```

## Performance

- Cache `.myks/vendir-cache/` between runs, as above. With `lazy: true` sources,
  a warm cache means no downloads at all.
- Limit parallelism on small runners with `--async` (or `MYKS_ASYNC`); the
  default runs every application at once.
- `--print-stats` prints how much CPU and memory each tool used, to find the
  expensive applications.

See [Optimizations](optimizations.md).
