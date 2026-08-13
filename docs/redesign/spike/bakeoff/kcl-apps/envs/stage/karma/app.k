"""karma — L1 leaf override (replicas), L6 any-passthrough config. Computed helm.values
depend on env clusterFullName, so they are layered at the env scope (envs/stage/env.k)."""

import lib.prototypes as p

app = p.Karma {
    name = "karma", proto = "karma", namespace = "karma", monitored = True
    replicas = 2                                            # L1: beats prototype default 1
    config = {
        ui.refresh = "30s"
        alertmanager = [{name = "central", uri = "http://10.0.0.1:9093"}]
    }
}
