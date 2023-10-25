# Example folder

This folder includes two example myks repositories with some explanation including the rendered output.

- [simple](simple/readme.md) example with two environments and one helm chart.
- [default](default/readme.md) example created with `myks init`

The rendered output is verified in gotests.

To ensure a stable rendering output, the targetRevision was configured to main (`envs/env-data.ytt.yaml``)

```yaml
argocd:
  app:
    source:
      #! render all argo apps with targetRevision: main
      targetRevision: main
```