// ROOT env level — base every cluster inherits.
local lib = import '../lib.libsonnet';
{
  id: '',
  kubernetesDistribution: 'gke',
  rancherEnabled: false,
  runsGrafanaOperator: false,
  helm: { common: { global: {
    clusterGroup: '',
    tier: '',
    region: '',
    octet: 0,
  } } },
  // CreateNamespace: null is simply omitted (null-to-remove is trivial in-language).
  argocd: { app: { syncPolicy: { syncOptions: { ServerSideApply: 'true' } } } },
  apps: [
    lib.app('bootstrap', 'bootstrap'),
    lib.app('cilium', 'cilium', 'kube-system'),
    lib.app('namespaces', 'namespaces', 'default'),
  ],
}
