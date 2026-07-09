// Leaf-level app instances (added by the stage env). Package auto-unifies, so this is
// just another file in the same scope — no import line, no roster registration boilerplate.
// Adding an app = add one struct here (and one entry to _leafApps below). ~3-6 lines.
package bakeoff

_karmaBase: #Karma & {
	name:      "karma"
	namespace: "karma"
	monitored: true
	replicas:  2 // L1: leaf override beats prototype default 1
	config: {
		ui: refresh: "30s"
		alertmanager: [{name: "central", uri: "http://10.0.0.1:9093"}]
	}
}

_cf: #CentralForwarder & {
	name:      "central-forwarder"
	namespace: "monitoring"
	monitored: true
	centralForwarder: #CF & {
		mode:    "vm-agent"
		enabled: true
		shards:  4
		remoteWrite: url: "http://10.0.0.1:8427/api/v1/write"
	}
}

_arc: #Arc & {name: "arc", namespace: "arc-system"}

_alert: #Alertmanager & {name: "alertmanager", namespace: "monitoring", monitored: true}

_dash: #App & {name: "karma-dashboards", proto: "dashboards"} // namespace via L4a in env.cue

_leafApps: [_karmaBase, _cf, _arc, _alert, _dash]
