// Env inheritance levels root -> onprem -> tools -> stage, resolved into one env scope.
//
// KEY CUE FINDING — override modeling. CUE unification only NARROWS; it cannot
// override a concrete value (`x:1 & x:2` is an error). So last-wins inheritance is
// NOT struct unification. The clean idiom: each overridable scalar is a list of
// per-level values, and `#last` selects the deepest one set. Override is explicit and
// one-line per field. Lists (apps) compose by concatenation + a remove-filter. This is
// data-flow, not mutation — no re-validation of intermediate states (a KCL papercut here).
package bakeoff

import "list"

// last-wins: deepest level that sets the field wins. Levels list shallow..deep.
#last: {_v: [_, ...], out: _v[len(_v)-1]}

// Per-level scalar values (shallow -> deep). Only levels that set a field list it;
// the last entry is the resolved value. Reads as a vertical override history.
_levels: {
	kubernetesDistribution: (#last & {_v: ["gke", "rke2"]}).out             // root, onprem
	rancherEnabled:         (#last & {_v: [false, true]}).out               // root, onprem
	clusterGroup:           (#last & {_v: ["tools"]}).out                   // tools
	region:                 (#last & {_v: ["europe-dc1"]}).out              // tools
	tier:                   (#last & {_v: ["stage"]}).out                   // stage
	octet:                  (#last & {_v: [20]}).out                        // stage
	id:                     (#last & {_v: ["tools-stage-eu-dc1"]}).out      // stage
}

// argocd: root sets it; CreateNamespace: null is simply never written (drop-by-omission).
_argocd: app: syncPolicy: syncOptions: ServerSideApply: "true"

// App roster composed across levels. Each level contributes a list; bootstrap (root)
// is removed at the leaf by filtering it out — list "override" = concat then filter.
_rootApps: [
	{name: "bootstrap", proto: "bootstrap"},
	#App & {name: "cilium", proto: "cilium", namespace: "kube-system"},
	#App & {name: "namespaces", proto: "namespaces", namespace: "default"},
]
_onpremApps: [
	#App & {name: "goldpinger", proto: "goldpinger", namespace: "monitoring", monitored: true},
	#App & {name: "cluster-autoscaler", proto: "cluster-autoscaler", namespace: "kube-system"},
]
_toolsApps: [
	#GrafanaOperator & {name: "grafana-operator", namespace: "grafana", monitored: true},
	#App & {name: "knot", proto: "knot", namespace: "knot"},
]

// Leaf apps live in apps.cue (one file per concern is just as cheap — package auto-unifies).
// _leafApps is defined there; here we only compose the roster.
_allApps: list.Concat([_rootApps, _onpremApps, _toolsApps, _leafApps])
_roster: [for a in _allApps if a.proto != "bootstrap" {a}] // remove bootstrap
