# Example folder

This folder includes two example myks repositories with some explanation including the rendered output.

- [simple](simple/readme.md) example with two environments and one helm chart.
- [default](default/readme.md) example created with `myks init`

The rendered output is verified in gotests.

To ensure a stable rendering output, the branches and remote names were configured to static values in `envs/env-data.ytt.yaml`

```yaml
argocd:
  app:
    source:
      #! Fixed config to run tests successfull in pipeline
      targetRevision: main
      repoURL: git@github.com:mykso/myks.git

#! Fixed git config to run tests successfull in pipeline.
myks:
  gitRepoBranch: "main"
  gitRepoUrl: "git@github.com:mykso/myks.git"
```
