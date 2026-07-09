"""
KCL entrypoint — composes ONE Application's data-values struct, the KCL replacement
for `ytt --data-values-inspect`. Host passes the ordered file list + context via -D:

  kcl run compose.k schema.k merge.k \
    -D paths="a.yaml,b.yaml,..." -D proto=karma -D removeProtos=bootstrap

Mirrors baseline.sh's ytt output (compare with: kcl ... | yq -P 'sort_keys(..)').
"""

import yaml
import merge
import schema

# --- host-provided context (discover.sh enumerates; KCL is not a FS walker) ---
_paths = option("paths").split(",")
_proto = option("proto")
_removeProtos = [p for p in option("removeProtos").split(",") if p != ""]

# --- compose: fold the ordered layers (engine handles the |-conflict, see merge.k) ---
_merged: {str:any} = merge.fold(_paths)
_env = _merged.environment
_g = _env.helm.common.global

# --- derive (edges 5, 8) ---
_apps = merge.removeApps(_env.applications, _removeProtos)  # edge 1 remove-by-key
_runsGO = any_true([a.proto == "grafana-operator" for a in _apps])  # edge 5 shared key

# --- type + validate; Global derives clusterFullName/clusterAddress (edge 8) by
#     dropping the hand-written values so the schema defaults compute them ---
_global = schema.Global {
    clusterGroup = _g.clusterGroup
    tier = _g.tier
    region = _g.region
    octet = _g.octet
}

environment: schema.Environment = {
    id = _env.id
    kubernetesDistribution = _env.kubernetesDistribution
    rancherEnabled = _env.rancherEnabled
    runsGrafanaOperator = _runsGO
    helm.common.global = _global
    applications = _apps
}

# Per-proto application schema supplies prototype defaults (edge 3) + validates
# (edge 4). Edge 6: `config` rides through as any. Karma -> port default; the
# central-forwarder -> one_of/min/conditional checks fire on cast.
_app = _merged.application
application = schema.Karma {**_app} if _proto == "karma" else \
              ({centralForwarder = schema.CentralForwarder {**_app.centralForwarder}} if _proto == "central-forwarder" else _app)

# passthrough (edge 7 null-to-remove lives here)
argocd = _merged.argocd
constants = {region_short = schema.getRegionShort()}
