"""ROOT env level — base every cluster inherits."""

import lib.schema as s

env = {
    id = ""
    kubernetesDistribution = "gke"
    rancherEnabled = False
    runsGrafanaOperator = False
    helm.common.global = s.Global {}
    # CreateNamespace: null is simply omitted (null-to-remove is trivial when composing in-language).
    argocd.app.syncPolicy.syncOptions = {ServerSideApply = "true"}
    apps = [
        s.App {name = "bootstrap", proto = "bootstrap"}
        s.App {name = "cilium", proto = "cilium", namespace = "kube-system"}
        s.App {name = "namespaces", proto = "namespaces", namespace = "default"}
    ]
}
