"""central-forwarder — L7 dense validation, L3 vendir from enabled flag.
grafanaIntegration (L4c) is set at the env scope from runsGrafanaOperator."""

import lib.prototypes as p

app = p.CentralForwarder {
    name = "central-forwarder", proto = "central-forwarder", namespace = "monitoring", monitored = True
    centralForwarder = p.CF {
        mode = "vm-agent", enabled = True, shards = 4
        remoteWrite = p.RemoteWrite {url = "http://10.0.0.1:8427/api/v1/write"}
    }
}
