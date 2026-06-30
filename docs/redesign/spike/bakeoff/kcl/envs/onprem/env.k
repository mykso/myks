"""CLUSTER GROUP: on-prem — overrides distro + rancher, appends two infra apps."""

import envs.root as parent
import lib.schema as s

env = parent.env {
    kubernetesDistribution = "rke2"     # L1 override gke -> rke2
    rancherEnabled = True
    apps += [
        s.App {name = "goldpinger", proto = "goldpinger", namespace = "monitoring", monitored = True}
        s.App {name = "cluster-autoscaler", proto = "cluster-autoscaler", namespace = "kube-system"}
    ]
}
