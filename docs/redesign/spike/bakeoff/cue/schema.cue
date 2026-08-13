// Schemas + prototypes (package auto-unifies with every other *.cue file here).
package bakeoff

// region -> short code. Leading _ keeps it private (not emitted).
_regionShort: {
	"europe-dc1":      "eu-dc1"
	"europe-west4":    "eu-w4"
	"us-central1":     "us-c1"
	"asia-southeast1": "as-se1"
}

// Env-global helm block. clusterFullName/clusterAddress are DERIVED (L7).
// Unlike KCL, CUE never re-evaluates on override (no mutation), so no intermediate-state
// guards are needed: the derivation reads the final unified values exactly once.
#Global: {
	clusterGroup: string
	tier:         string
	region:       string
	octet:        int
	// L7 derived. region must be in the map (acts as the L7 validation check).
	clusterFullName: "\(clusterGroup)-\(tier)-\(_regionShort[region])"
	clusterAddress:  "10.0.0.\(octet)"
}

// Base application. namespace defaults to name; monitored is the L4b metadata mark.
#App: {
	name:      string
	proto:     string
	namespace: string | *name
	monitored: bool | *false
	...
}

// --- Prototypes: typed defaults an app extends (the "app amends prototype" edge). ---

// karma — L7 typed defaults, L6 any-passthrough config, computed helm values.
#Karma: #App & {
	proto:    "karma"
	port:     int | *8080
	image:    string | *"registry.example.com/ghcr-io-mirror/prymitive/karma:v0.131"
	replicas: int | *1
	config?: _
	helm?: _
}

#RemoteWrite: {url: string | *""}

#CF: {
	mode:                "prometheus" | "prometheus-agent" | *"vm-agent" // one_of
	enabled:             bool | *false
	shards:              int & >=1 | *2 // min=1
	alertmanagerVersion: _ | *"v0.27.0" // nullable
	remoteWrite:         #RemoteWrite | *{}
	// L7 conditional: URL required when enabled.
	if enabled {remoteWrite: url: string & !=""}
}

// central-forwarder — L7 dense validation + L3 vendir derived from enabled flag.
#CentralForwarder: #App & {
	proto:             "central-forwarder"
	centralForwarder:  #CF
	grafanaIntegration: bool | *false // L4c: set from env at the leaf
	// L3: vendir source set derived from enabled (one source of truth, no double-toggle).
	vendir: sources: {
		namespace: helmChart: {name: "namespace", url: "https://charts.example.com", version: "3.0.0"}
		if centralForwarder.enabled {
			"vm-agent": helmChart: {name: "victoria-metrics-agent", url: "https://victoriametrics.github.io/helm-charts", version: "0.14.0"}
		}
	}
}

// grafana-operator — presence drives L4c.
#GrafanaOperator: #App & {
	proto: "grafana-operator"
	image: string | *"ghcr.io/grafana/grafana-operator:v5.20.0"
}

// arc — L2 self-ref: one namespace feeds two keys. Static helm = plain-YAML passthrough.
#Arc: #App & {
	proto:               "arc"
	namespace:           string | *"arc-system"
	controllerNamespace: namespace
	runnerNamespace:     namespace
	helm: values: {githubConfigUrl: "https://github.com/myorg", minRunners: 1, maxRunners: 10}
	vendir: sources: {
		controller: helmChart: {name: "gha-runner-scale-set-controller", url: "https://actions.github.io/actions-runner-controller-charts", version: "0.9.3"}
		"runner-set": helmChart: {name: "gha-runner-scale-set", url: "https://actions.github.io/actions-runner-controller-charts", version: "0.9.3"}
	}
}

// alertmanager — L4b: routes computed at the leaf.
#Alertmanager: #App & {
	proto:  "alertmanager"
	routes: [...{receiver: string, namespace: string}] | *[]
}
