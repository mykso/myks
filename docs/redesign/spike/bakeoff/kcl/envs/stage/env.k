"""LEAF env: tools-stage-eu-dc1. Sets identity, assembles leaf apps, removes bootstrap,
derives env-scope flags, and resolves all cross-app introspection (L4a/b/c) in one scope."""

import envs.tools as parent
import lib.schema as s
import lib.prototypes as p

# --- env-scope overrides (apps stay a list while inheriting) ---
_lvl = parent.env {
    id = "tools-stage-eu-dc1"
    helm.common.global.tier = "stage"
    helm.common.global.octet = 20
}

# --- leaf apps: instantiate prototypes + per-app overrides (the _apps/<app> layer) ---
_karmaBase = p.Karma {
    name = "karma", proto = "karma", namespace = "karma", monitored = True
    replicas = 2                                            # L1: beats prototype default 1
    config = {
        ui.refresh = "30s"
        alertmanager = [{name = "central", uri = "http://10.0.0.1:9093"}]
    }
}
_cf = p.CentralForwarder {
    name = "central-forwarder", proto = "central-forwarder", namespace = "monitoring", monitored = True
    centralForwarder = p.CF {
        mode = "vm-agent", enabled = True, shards = 4
        remoteWrite = p.RemoteWrite {url = "http://10.0.0.1:8427/api/v1/write"}
    }
}
_arc = p.Arc {name = "arc", proto = "arc", namespace = "arc-system"}
_alert = p.Alertmanager {name = "alertmanager", proto = "alertmanager", namespace = "monitoring", monitored = True}
_dash = s.App {name = "karma-dashboards", proto = "dashboards"}   # namespace set via L4a below

# --- roster: drop bootstrap (remove-by-key during composition), append leaf apps ---
_appList = [a for a in _lvl.apps if a.proto != "bootstrap"] + [_karmaBase, _cf, _arc, _alert, _dash]

# --- env-scope derivation ---
_runsGO = any_true([a.proto == "grafana-operator" for a in _appList])     # L4c
_fullName = _lvl.helm.common.global.clusterFullName

# --- karma computed helm values (L: computed-helm; depends on env clusterFullName) ---
_karma = _karmaBase {
    helm.values = {
        replicaCount = _karmaBase.replicas
        image = _karmaBase.image
        external_url = "https://karma.${_fullName}.example.com"
    }
}

# --- cross-app introspection, one eval scope ---
_byName = {a.name: a for a in _appList}
_routeMap = {a.name: {receiver = a.name, namespace = a.namespace} for a in _appList if a.monitored and a.name != "alertmanager"}
_routes = [_routeMap[k] for k in sorted([k for k in _routeMap])]          # L4b, name-sorted

# Apps whose final config needs cross-app / computed values layered on the base instance.
_patched = {
    "karma" = _karma
    "karma-dashboards" = _dash {namespace = _byName["karma"].namespace}   # L4a: B targets A's ns
    "alertmanager" = _alert {routes = _routes}                            # L4b
    "central-forwarder" = _cf {grafanaIntegration = _runsGO}              # L4c
}
_apps = {a.name: (_patched[a.name] if a.name in _patched else a) for a in _appList}

env = {
    id = _lvl.id
    kubernetesDistribution = _lvl.kubernetesDistribution
    rancherEnabled = _lvl.rancherEnabled
    runsGrafanaOperator = _runsGO
    helm = _lvl.helm
    argocd = _lvl.argocd
    applications = _apps
}
