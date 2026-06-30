"""Prototype schemas (package `lib`). A prototype = typed defaults an app extends.
Each app instantiates its prototype schema and overrides at the leaf — this is the
"apps import their prototype" inheritance edge."""

import lib.schema as s

# karma — L7 typed defaults, L6 any-passthrough config. helm.values computed at the leaf.
schema Karma(s.App):
    port: int = 8080
    image: str = "registry.example.com/ghcr-io-mirror/prymitive/karma:v0.131"
    replicas: int = 1
    config?: any
    helm?: any

# central-forwarder — L7 dense validation surface.
schema RemoteWrite:
    url: str = ""

schema CF:
    mode: "prometheus" | "prometheus-agent" | "vm-agent" = "prometheus-agent"  # one_of
    enabled: bool = False
    shards: int = 2                                                            # min=1 below
    alertmanagerVersion?: any = "v0.27.0"                                      # nullable
    remoteWrite: RemoteWrite = RemoteWrite {}
    check:
        shards >= 1, "shards must be >= 1"
        len(remoteWrite.url) > 0 if enabled, "set the URL when enabling the forwarder"

schema CentralForwarder(s.App):
    centralForwarder: CF = CF {}
    grafanaIntegration: bool = False    # L4c: set from env runsGrafanaOperator at the leaf
    # L3: vendir source set derived from the enabled flag (one source of truth, no double-toggle).
    vendir: any = {
        sources = {
            namespace.helmChart = {name = "namespace", url = "https://charts.example.com", version = "3.0.0"}
        } | ({
            "vm-agent" = {helmChart = {name = "victoria-metrics-agent", url = "https://victoriametrics.github.io/helm-charts", version = "0.14.0"}}
        } if centralForwarder.enabled else {})
    }

# grafana-operator — its presence in the roster drives L4c.
schema GrafanaOperator(s.App):
    image: str = "ghcr.io/grafana/grafana-operator:v5.20.0"

# arc — L2 self-reference: one namespace feeds two keys. Static helm = plain-YAML passthrough.
schema Arc(s.App):
    namespace: str = "arc-system"
    controllerNamespace: str = namespace
    runnerNamespace: str = namespace
    helm: any = {
        values = {githubConfigUrl = "https://github.com/myorg", minRunners = 1, maxRunners = 10}
    }
    vendir: any = {
        sources = {
            controller = {helmChart = {name = "gha-runner-scale-set-controller", url = "https://actions.github.io/actions-runner-controller-charts", version = "0.9.3"}}
            "runner-set" = {helmChart = {name = "gha-runner-scale-set", url = "https://actions.github.io/actions-runner-controller-charts", version = "0.9.3"}}
        }
    }

# alertmanager — L4b: routes are a comprehension over every monitored app, computed at the leaf.
schema Alertmanager(s.App):
    routes: [{str:str}] = []
