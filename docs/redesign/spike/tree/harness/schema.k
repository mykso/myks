"""
Typed schemas + validation (edges 3, 4) and the derivation that ytt data values
cannot do (edges 5, 8). Applied ONCE to the composed result (spike finding #2:
`check` must not run on intermediate fold steps).
"""

# Constant lookup (root env-data.schema `constants.region_short`), kept in code.
# Underscore => module-private, so it is NOT emitted as a stray top-level output key.
_regionShort: {str:str} = {
    "europe-dc1" = "eu-dc1"
    "europe-west4" = "eu-w4"
    "us-central1" = "us-c1"
    "asia-southeast1" = "as-se1"
}
getRegionShort = lambda -> {str:str} { _regionShort }  # public accessor (lambdas aren't emitted)

# Edge 3/6: per-prototype application schema = typed defaults (port=8080) the
# prototype contributes. `config: any` is the edge-6 free-form passthrough.
schema Karma:
    port: int = 8080
    image: str = ""
    replicas: int = 1
    config?: any

schema App:
    proto: str
    name: str = ""  # ytt fills missing name with "" from the schema default

schema Global:
    clusterGroup: str = ""
    tier: str = ""
    region: str = ""
    octet: int = 0
    # Edge 8: DERIVED (ytt hand-writes both). Computed from sibling fields.
    clusterFullName: str = "${clusterGroup}-${tier}-${_regionShort[region]}"
    clusterAddress: str = "10.0.0.${octet}"
    check:
        region in _regionShort, "region '${region}' missing from _regionShort map"

# Edge 4: app-scope validation surface, modelled on prometheus-central-package.
schema RemoteWrite:
    url: str = ""

schema CentralForwarder:
    mode: "prometheus" | "prometheus-agent" | "vm-agent" = "prometheus-agent"  # one_of
    enabled: bool = False
    shards: int = 2
    # nullable: ytt `#@schema/nullable` defaults to null (type taken from the value).
    # KCL match = None default, which renders as `null`.
    alertmanagerVersion?: any = None
    remoteWrite: RemoteWrite = RemoteWrite {}
    check:
        shards >= 1, "shards must be >= 1"                       # min=1
        # conditional cross-field: url required WHEN enabled (ytt when=lambda)
        len(remoteWrite.url) > 0 if enabled, "set the URL when enabling the forwarder"

schema Common:
    global: Global

schema Helm:
    common: Common

# Edge 3: env-scope schema; `application` stays open/any (edge 6 passthrough).
schema Environment:
    id: str = ""
    kubernetesDistribution: "gke" | "rke2" | "k3s" = "gke"   # one_of
    rancherEnabled: bool = False
    runsGrafanaOperator: bool = False
    helm: Helm
    applications: [App] = []
