# prototype management

Prototypes can use [vendir](https://carvel.dev/vendir/) to manage the source files.
The vendir configuration is defined in `prototypes/_name_/vendir/` and on each level of environments `envs/_env_/_apps/_name_/vendir`.
The vendir configuration is rendered using ytt.

Myks expects vendir sync to store the files in a folder based on the kind of the prototype: `./(charts|ytt|static|ytt-pkg)/_name_`.

## self-managed

The vendir config file generation can be managed manually.
A common pattern is to use to split the config into data `vendir-data.ytt.yaml` and template `base.ytt.yaml` using `application` as key for the config settings.

```yaml
#! filename: vendir-data.ytt.yaml
#@data/values-schema
---
#@overlay/match-child-defaults missing_ok=True
application:
  #! WARNING: The order of the keys (alphabetical) is important for renovate.
  #!          When changed, renovate won't be able to detect the new version.
  #!          See renovate.json for more details.
  #! renovate: datasource=helm
  name: httpbingo
  url: https://estahn.github.io/charts
  version: 0.1.0
```

```yaml
#! filename: base.ytt.yaml
#@ load("@ytt:data", "data")

#@ app = data.values.application
---
apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
  - path: #@ "charts/" + app.name
    contents:
      - path: .
        helmChart:
          name: #@ app.name
          version: #@ app.version
          repository:
            url: #@ app.url
```
These files can be adjusted as needed for each prototype.

## myks managed

Myks provides a flexible way of managing prototypes with the most common vendir options available.

Create prototype:
```shell
myks proto add -p myApp
myks proto add -p anotherApp/frontend
myks proto add -p anotherApp/backend
```
This created a the skeleton. It is still required to add sources for the applications.

Add Sources to a prototype:
```shell
# Add httpbingo source to our myApp prototype.
myks proto src add --prototype myApp --name httpbingo --url https://estahn.github.io/charts --version 0.1.0

# `--create` will create the prototype if necessary.
myks proto src add --create -p httpbingo -n httpbingo -u https://estahn.github.io/charts -v 0.1.0

# By default a helm chart is added from a chart repository.
# It is also possible to add files from a git repository.
myks proto src add --create -p argocd -n argocd --url https://github.com/argoproj/argo-cd --version v2.7.3 --kind ytt --repo git --include manifests/ha/install.yaml --rootPath manifests/ha
```
This will generate `vendir-data.ytt.yaml` with the following content for argocd:
```yaml
#! This file is managed by myks
#@data/values
---
#@overlay/match-child-defaults missing_ok=True
prototypes:
  - name: argocd
    kind: ytt
    repo: git
    url: https://github.com/argoproj/argo-cd
    version: v2.7.3
    newRootPath: manifests/ha
    includePaths:
      - manifests/ha/install.yaml
```

It is usually safe to edit this file if following the schema. On each render operation myks will export the schema in `.myks/tmp/data-schema.ytt.yaml`.

Delete a src:
```shell
myks proto src delete -p myApp -n httpbingo
```

Delete a prototype:
```shell
myks proto delete -p myApp
```