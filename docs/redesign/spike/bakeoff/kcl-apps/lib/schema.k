"""Base schemas shared by every env level (package `lib`)."""

# region -> short code (env-scope constant, kept private so it is not emitted).
_regionShort: {str:str} = {
    "europe-dc1" = "eu-dc1"
    "europe-west4" = "eu-w4"
    "us-central1" = "us-c1"
    "asia-southeast1" = "as-se1"
}

# Env-global helm block. clusterFullName/clusterAddress are DERIVED (L7) — ytt hand-writes them.
# Guarded so intermediate levels (empty region) stay valid: KCL re-evaluates derived
# defaults + checks on every config override, so they must tolerate the not-yet-set state.
schema Global:
    clusterGroup: str = ""
    tier: str = ""
    region: str = ""
    octet: int = 0
    clusterFullName: str = "${clusterGroup}-${tier}-${_regionShort[region]}" if region in _regionShort else ""
    clusterAddress: str = "10.0.0.${octet}"
    check:
        region == "" or region in _regionShort, "region '${region}' missing from regionShort map"

# Base application. namespace defaults to the app name; monitored is the L4b metadata mark.
schema App:
    name: str
    proto: str
    namespace: str = name
    monitored: bool = False
