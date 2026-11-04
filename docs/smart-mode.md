# Smart Mode

Smart Mode looks at the files you changed and processes only the environments
and applications those changes can affect. It is what makes `myks render`
without arguments fast on a large tree, locally and in CI.

Smart Mode is on whenever you run `myks render` or a [plugin](plugins.md)
without positional arguments. Any selector (`myks render ALL`,
`myks render prod`) turns it off.

## How it works

Smart Mode asks Git for the changed files:

- always: `git status --untracked-files`, that is staged, unstaged and untracked
  changes;
- with `--smart-mode.base-revision=<rev>`: also `git diff <rev>`, every file
  that differs between `<rev>` and the working tree.

It then maps every changed path to environments and applications with the rules
below. Decisions are based on paths, not on file content, so Smart Mode
sometimes processes more than strictly necessary.

When nothing is affected (for example, only a `README.md` changed), myks exits
before syncing or rendering anything. When Smart Mode cannot run — the project
is not in a Git repository, Git fails, or the project uses
[KCL mode](kcl-mode.md) — myks logs a warning and processes everything; with
`--smart-mode.only-print`, it fails instead.

## Rules

Paths use the default [naming conventions](configuration.md#naming-conventions).
`<dir>` stands for any of the per-application directories: `vendir`, `lib`,
`helm`, `ytt`, `ytt-pkg`, `static`, `argocd`.

### Everything

- `lib/**` — the root ytt library
- `prototypes/_vendir/**` — the shared vendir base, see [sources.md](sources.md)

### All applications of environments

A change at `envs/<path>/` selects every environment at or below `envs/<path>/`,
with all its applications:

- `envs/<path>/env-data*.yaml`
- `envs/<path>/_env/ytt/**`
- `envs/<path>/_env/argocd/**`
- `envs/<path>/_env/static/**`

A change in `envs/env-data.ytt.yaml` therefore selects every environment.

### Applications of a prototype

- `prototypes/<proto>/app-data*.yaml` and `prototypes/<proto>/<dir>/**` select
  every application based on `<proto>`, in every environment.
- `envs/<path>/_proto/<proto>/app-data*.yaml` and
  `envs/<path>/_proto/<proto>/<dir>/**` select the applications based on
  `<proto>` in environments at or below `envs/<path>/`.

### Single applications

- `envs/<path>/_apps/<app>/app-data*.yaml` and
  `envs/<path>/_apps/<app>/<dir>/**` select `<app>` in the environments at or
  below `envs/<path>/` that have it.
- `rendered/envs/<env-id>/<app>/**` and
  `rendered/argocd/<env-id>/app-<app>.yaml` select `<app>` in that environment,
  so hand-made edits to rendered output are rendered over.
- An application listed in an environment but missing from
  `rendered/envs/<env-id>/` is selected, so new applications get rendered.

### Not watched

Changes to anything else select nothing, including `.myks.yaml` and files
outside `envs/`, `prototypes/`, `lib/` and `rendered/`. Render the affected
environments explicitly after changing them, for example
`myks render prod`.

## Sync only

With `myks render --sync`, Smart Mode ignores changes that can only affect
rendering, since they cannot change what gets downloaded:

- `_env/ytt/`, `_env/argocd/` and `_env/static/` directories;
- application and prototype directories other than `vendir/` and `lib/`;
- rendered output, and applications missing from it.

Data-values files, `vendir/` and `lib/` directories, `prototypes/_vendir/` and
the root `lib/` still count, because they feed the vendir configuration.

## Previewing the selection

`--smart-mode.only-print` prints what would be processed and exits:

```console
$ myks render --smart-mode.only-print

Smart Mode detected:
→ envs/prod/eu
    frontend
→ envs/stage
    ALL
```

```console
$ myks render --smart-mode.only-print=json
{
  "envs/prod/eu": [
    "frontend"
  ],
  "envs/stage": null
}
```

Keys are environment directories; `null` and `ALL` mean all applications of
every environment at or below that directory. The flag also works with explicit
selectors, which makes it a quick way to check what `myks render prod frontend`
would touch.

## Smart Mode in CI

Uncommitted changes are rarely what CI should look at. Compare with the branch
the change goes into:

```shell
myks render --smart-mode.base-revision=origin/main
```

The revision must exist in the checkout, so fetch enough history (for example,
`fetch-depth: 0` with `actions/checkout`). See [gitops-ci.md](gitops-ci.md) for
complete pipelines.
