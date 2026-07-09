// CUE — replace ytt's data-values COMPOSITION (not its templates).
// Same tree as the KCL/ytt spikes (root -> on-prem -> tools -> stage -> prototype).
// Run: cue eval compose.cue -e environment
//
// CUE composes by UNIFICATION (&). Fields set by exactly one layer merge for free
// (clusterGroup, region, tier below). Two frictions the idiom must work around:
//   * Last-wins override: two layers setting the SAME scalar concretely is a CONFLICT,
//     not an override. The base must offer a *default (kubernetesDistribution, rancherEnabled)
//     so a later concrete can win. A plain concrete base would error — see README.
//   * Lists do not append under &. applications is concatenated explicitly (list.Concat),
//     separate from the unification, because each layer carries a different list.
package compose

import "list"

#App: {name: string}

#Global: {
	clusterGroup: string
	tier:         string
	region:       string
	region_short: [string]: string
	region_short: {
		"europe-dc1":     "eu-dc1"
		"europe-west4":    "eu-w4"
		"us-central1":     "us-c1"
		"asia-southeast1": "as-se1"
	}
	// DERIVED self-reference.
	clusterFullName: "\(clusterGroup)-\(tier)-\(region_short[region])"
}

#Env: {
	kubernetesDistribution: *"gke" | "rke2" | "k3s"
	rancherEnabled:         *false | bool
	helm: common: global: #Global
	applications: [...#App]
	// DERIVED shared key.
	runsGrafanaOperator: list.Contains([for a in applications {a.name}], "grafana-operator")
}

// --- layers (scalar/struct parts unify; lists handled separately) ---
_root:   #Env & {}
_onprem: {kubernetesDistribution: "rke2", rancherEnabled: true}
_tools:  {helm: common: global: {clusterGroup: "tools", region: "europe-dc1"}}
_stage:  {helm: common: global: {tier:                                 "stage"}}

_appLayers: [
	[{name: "cilium"}, {name: "namespaces"}],
	[{name: "goldpinger"}, {name: "cluster-autoscaler"}],
	[{name: "grafana-operator"}, {name: "knot"}],
	[{name: "karma"}],
]

environment: #Env & _root & _onprem & _tools & _stage & {
	applications: list.Concat(_appLayers)
}
