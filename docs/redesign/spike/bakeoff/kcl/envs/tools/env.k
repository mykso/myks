"""TIER: tools — sets cluster identity, appends grafana-operator (drives L4c) + knot."""

import envs.onprem as parent
import lib.schema as s
import lib.prototypes as p

env = parent.env {
    helm.common.global.clusterGroup = "tools"
    helm.common.global.region = "europe-dc1"
    apps += [
        p.GrafanaOperator {name = "grafana-operator", proto = "grafana-operator", namespace = "grafana", monitored = True}
        s.App {name = "knot", proto = "knot", namespace = "knot"}
    ]
}
