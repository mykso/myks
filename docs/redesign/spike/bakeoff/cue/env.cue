// Whole-tree assembly + cross-app introspection (L4a/b/c), one eval scope.
package bakeoff

import "list"

_global: #Global & {
	clusterGroup: _levels.clusterGroup
	tier:         _levels.tier
	region:       _levels.region
	octet:        _levels.octet
}

// L4c: grafana-operator present in roster.
_runsGO: list.Contains([for a in _roster {a.proto}], "grafana-operator")

// by-name index over the roster (for L4a).
_byName: {for a in _roster {(a.name): a}}

// L4b: routes = monitored apps (self excluded), sorted by name.
_routes: [
	for n in list.Sort([for a in _roster if a.monitored && a.name != "alertmanager" {a.name}], list.Ascending)
	{receiver: n, namespace: _byName[n].namespace},
]

// Per-app patches layering computed/cross-app values onto the base instances.
_patched: {
	karma: _karmaBase & {
		helm: values: {
			replicaCount: _karmaBase.replicas
			image:        _karmaBase.image
			external_url: "https://karma.\(_global.clusterFullName).example.com"
		}
	}
	"karma-dashboards": _dash & {namespace: _byName.karma.namespace} // L4a
	alertmanager:       _alert & {routes:    _routes}                // L4b
	"central-forwarder": _cf & {grafanaIntegration: _runsGO}         // L4c
}

// applications map: patched instance where one exists, else the roster entry.
_applications: {
	for a in _roster {
		(a.name): [if _patched[a.name] != _|_ {_patched[a.name]}, a][0]
	}
}

environment: {
	id:                     _levels.id
	kubernetesDistribution: _levels.kubernetesDistribution
	rancherEnabled:         _levels.rancherEnabled
	runsGrafanaOperator:    _runsGO
	helm: common: global: _global
	argocd:       _argocd
	applications: _applications
}
