# Why myks

Myks exists for one workflow: you manage many clusters from Git, most of what
runs on them is third-party software, and you want to review the exact manifests
before they are deployed. This page explains that workflow, what myks adds to
it, and how it compares with tools you may already use.

## Rendered manifests in Git

Most GitOps setups store _inputs_ in Git (charts, values, overlays) and let the
deployment tool render them at sync time. Myks renders ahead of time and stores
the _output_ in Git too, an approach often called the rendered manifests
pattern:

1. You change an input: a values file, a chart version, an overlay.
2. `myks render` (locally or in CI) updates `rendered/`.
3. The pull request contains both. Reviewers see which resources change in which
   cluster, field by field.
4. After the merge, Argo CD, Flux or any other tool applies plain YAML. It needs
   no Helm, no plugins and no access to chart repositories.

What you get from it:

- **Reviews of real changes.** A chart upgrade that silently changes a default,
  a values typo that drops a container port, a refactoring that should change
  nothing: the diff shows it before the merge.
- **History and rollback.** `git log rendered/envs/prod-eu/` is the deployment
  history of a cluster. Reverting a commit reverts the manifests exactly.
- **Simple, fast deployment.** The deployment tool applies static files, with
  nothing to template or download at sync time.
- **No lock-in.** The output is plain Kubernetes YAML. If you stop using myks,
  the rendered manifests keep working with any tool.

The cost is a bigger repository and the need to keep `rendered/` in sync with
its inputs. Myks keeps the output deterministic, one file per resource, and only
re-renders what changed; [GitOps and CI](gitops-ci.md) shows how to keep it in
sync automatically.

## What myks adds

Rendering many applications for many clusters needs more than a templating
engine. Myks covers the whole job:

| Need                                      | How myks covers it                                                                             |
| ----------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Fetch Helm charts, Git trees, files       | [vendir](sources.md): Helm and OCI repositories, Git, GitHub releases, HTTP                    |
| Avoid downloading the same thing twice    | Shared cache in `.myks/vendir-cache/`, lazy syncing                                            |
| Render hundreds of applications quickly   | Parallel processing, [Smart Mode](smart-mode.md) renders only what changed                     |
| Configure clusters without copy-paste     | [Inheritance](concepts.md#inheritance) over a directory tree of any depth, reusable prototypes |
| Change upstream charts without forking    | [ytt overlays](ytt.md) per application, per prototype, per environment                         |
| Reviewable output                         | Sorted keys, one file per resource, stale files removed                                        |
| Argo CD integration                       | [Generated](argocd.md) `Application`, `AppProject` and cluster `Secret`                        |
| Image pinning and registry mirrors        | [kbld](kbld.md): tags to digests, registry and repository rewrites                             |
| Understand where configuration comes from | [`myks inspect`](inspect.md): file lists, value provenance, redundant overrides                |
| Custom checks and deployment              | [Plugins](plugins.md) that run per application                                                 |

A production installation renders about 680 Argo CD Applications for 25 clusters
from 61 prototypes, about 22,500 manifest files.

## Compared with other tools

The comparison below comes from the landscape survey behind the
[configuration-layer redesign](redesign/research-findings.md) (2025–2026). It
rates each tool against the eight capabilities above that a myks installation
relies on.

| Tool              | Fetch sources | Cache | Parallel render | Tree inheritance | Sliced output to Git | Argo CD generation | Image mirroring | Pluggable steps |
| ----------------- | :-----------: | :---: | :-------------: | :--------------: | :------------------: | :----------------: | :-------------: | :-------------: |
| **myks**          |       ●       |   ●   |        ●        |        ●         |          ●           |         ●          |        ●        |        ●        |
| Holos             |       ◐       |   ●   |        ●        |        ●         |          ◐           |         ●          |        ○        |        ◐        |
| Helmfile          |       ●       |   ●   |        ●        |        ●         |          ●           |         ◐          |        ◐        |        ◐        |
| Kustomize         |       ●       |   ○   |        ○        |        ●         |          ◐           |         ○          |        ●        |        ◐        |
| kluctl            |       ●       |   ◐   |        ●        |        ●         |          ◐           |         ○          |        ◐        |        ○        |
| Timoni            |       ◐       |   ●   |        ○        |        ◐         |          ○           |         ○          |        ○        |        ○        |
| cdk8s             |       ◐       |   ○   |        ○        |        ◐         |          ●           |         ◐          |        ○        |        ○        |
| Argo CD plugins\* |       ●       |   ○   |       n/a       |       n/a        |          ○           |         ◐          |       n/a       |        ●        |

● native · ◐ partial, or buildable on top · ○ none. \*Config management plugins
render at sync time, not to Git.

In short:

- **Helm alone** renders one chart with layered values files. Myks uses Helm for
  that, and adds the tree of environments, the overlays for changes a chart does
  not expose, and the output in Git.
- **Kustomize** is strong at patching, which myks covers with ytt overlays. It
  has no shared download cache, no parallel rendering, and no Argo CD
  generation.
- **Helmfile** is the closest fit for Helm-only setups. Image mirroring and Argo
  CD generation are partial and need work on top.
- **Holos** shares the architecture (render to Git, Argo CD generation,
  inheritance in CUE), without image mirroring or automatic per-resource output.
- **Argo CD ApplicationSets and config management plugins** template at sync
  time. Reviewers see inputs, not manifests.

## Typical workflows

**Inspecting what an upstream chart contains.** After a render, every source is
unpacked under `.myks/envs/<env>/_apps/<app>/vendor/`. No `helm pull`, no
temporary directories.

**Changing a third-party chart.** Instead of forking it, releasing a patched
version and keeping the patch up to date, add a ytt overlay next to the
application. The rendered diff shows the effect immediately, and the overlay
survives chart upgrades as long as the resource it matches exists.

**Using sources that are not charts.** Dashboards in a Git repository, a CRD
bundle from a GitHub release, a single YAML file from a URL: add a vendir source
and, if needed, a ytt template around it. No wrapper chart.

**Rendering only what changed.** Locally, myks compares the working tree with
`HEAD`; in CI, with the target branch. A change to one application in one
cluster renders that application only; a change to a shared level renders every
environment below it.

## Limitations

- Myks does not connect to clusters, build images, or decrypt secrets.
- Data values use [ytt](https://carvel.dev/ytt/), which takes time to learn. A
  typed configuration layer in KCL is in preview ([KCL mode](kcl-mode.md), and
  [why KCL](why-kcl.md)).
- Committing rendered output makes large upgrades produce large diffs. Myks
  keeps them minimal, but a chart major version can still touch hundreds of
  files.
- Myks is a small open-source project. The output being plain YAML keeps the
  risk of adopting it low.
