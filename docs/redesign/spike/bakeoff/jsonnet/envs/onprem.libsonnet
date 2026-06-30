// CLUSTER GROUP: on-prem — overrides distro + rancher, appends two infra apps.
local lib = import '../lib.libsonnet';
local parent = import 'root.libsonnet';
parent + {
  kubernetesDistribution: 'rke2',  // L1 override gke -> rke2
  rancherEnabled: true,
  // `+` replaces arrays, so append explicitly (the one list-merge tax).
  apps: parent.apps + [
    lib.app('goldpinger', 'goldpinger', 'monitoring', monitored=true),
    lib.app('cluster-autoscaler', 'cluster-autoscaler', 'kube-system'),
  ],
}
