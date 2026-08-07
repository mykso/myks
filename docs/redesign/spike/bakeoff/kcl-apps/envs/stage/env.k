"""LEAF env: tools-stage-eu-dc1. Sets identity, assembles the leaf roster from the
per-app subpackages, removes bootstrap, derives env-scope flags, and resolves all
cross-app introspection (L4a/b/c) in one scope.

Per-app layout: each leaf app is its own subpackage (karma/, central_forwarder/, arc/,
alertmanager/, karma_dashboards/), each exporting `app`. Adding an app = new dir + one
import line + one roster entry below. No subpackage auto-discovery in KCL, so the wiring
is explicit here."""

import envs.tools as parent
import lib.schema as s
import lib.prototypes as p
import envs.stage.karma as karma
import envs.stage.central_forwarder as central_forwarder
import envs.stage.arc as arc
import envs.stage.alertmanager as alertmanager
import envs.stage.karma_dashboards as karma_dashboards

# --- env-scope overrides (apps stay a list while inheriting) ---
_lvl = parent.env {
    id = "tools-stage-eu-dc1"
    helm.common.global.tier = "stage"
    helm.common.global.octet = 20
}

# --- leaf roster: drop bootstrap (remove-by-key during composition), append the per-app instances ---
_appList = [a for a in _lvl.apps if a.proto != "bootstrap"] + [
    karma.app, central_forwarder.app, arc.app, alertmanager.app, karma_dashboards.app
]

# --- env-scope derivation ---
_runsGO = any_true([a.proto == "grafana-operator" for a in _appList])     # L4c
_fullName = _lvl.helm.common.global.clusterFullName

# --- karma computed helm values (L: computed-helm; depends on env clusterFullName) ---
_karma = karma.app {
    helm.values = {
        replicaCount = karma.app.replicas
        image = karma.app.image
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
    "karma-dashboards" = karma_dashboards.app {namespace = _byName["karma"].namespace}   # L4a: B targets A's ns
    "alertmanager" = alertmanager.app {routes = _routes}                                 # L4b
    "central-forwarder" = central_forwarder.app {grafanaIntegration = _runsGO}           # L4c
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
