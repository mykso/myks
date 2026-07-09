# KCL — replace ytt's data-values COMPOSITION (not its templates).
# Models the inheritance tree myks feeds ytt (root -> on-prem -> tools -> stage -> prototype)
# and produces the one merged struct, typed + validated, with clusterFullName and
# runsGrafanaOperator DERIVED (self-reference ytt data values cannot do). Run: kcl run compose.k
#
# Idiom forced by KCL (see spike README "KCL composition findings"):
#   1. fold layers as an {str:any} dict accumulator  — plain dicts deep-merge under `|`;
#      nested *schema* instances do NOT (they get replaced), so we cannot fold typed values.
#   2. cast ONCE to the typed schema at the end       — typing, `check`, and derived
#      attributes then run on the composed result, not on each incomplete intermediate.

schema App:
    name: str

schema Global:
    clusterGroup?: str
    tier?: str
    region?: str
    region_short: {str:str} = {
        "europe-dc1" = "eu-dc1"
        "europe-west4" = "eu-w4"
        "us-central1" = "us-c1"
        "asia-southeast1" = "as-se1"
    }
    # DERIVED self-reference — hand-typed (and drift-prone) in ytt data values.
    clusterFullName: str = "${clusterGroup}-${tier}-${region_short[region]}"

schema Common:
    global: Global

schema Helm:
    common: Common

schema Environment:
    kubernetesDistribution: "gke" | "rke2" | "k3s" = "gke"
    rancherEnabled: bool = False
    applications: [App] = []
    helm: Helm
    # DERIVED shared key — kills the hand-set runsGrafanaOperator double-toggle.
    runsGrafanaOperator: bool = any_true([a.name == "grafana-operator" for a in applications])
    check:
        helm.common.global.region in helm.common.global.region_short, \
            "region '${helm.common.global.region}' missing from region_short map"

# --- compose the tree: fold env-data layers as an untyped dict accumulator ---
_d: {str:any} = {kubernetesDistribution = "gke", rancherEnabled = False, applications = []}  # root
_d = _d | {applications += [{name = "cilium"}, {name = "namespaces"}]}
_d = _d | {kubernetesDistribution = "rke2", rancherEnabled = True, \
           applications += [{name = "goldpinger"}, {name = "cluster-autoscaler"}]}            # on-prem
_d = _d | {helm.common.global.clusterGroup = "tools", helm.common.global.region = "europe-dc1", \
           applications += [{name = "grafana-operator"}, {name = "knot"}]}                    # tools
_d = _d | {helm.common.global.tier = "stage", applications += [{name = "karma"}]}             # stage

# single cast: typing + validation + derived values all apply to the composed result
environment: Environment = _d
