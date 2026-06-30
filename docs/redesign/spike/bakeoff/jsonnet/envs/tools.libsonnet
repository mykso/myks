// TIER: tools — sets cluster identity, appends grafana-operator (drives L4c) + knot.
local lib = import '../lib.libsonnet';
local parent = import 'onprem.libsonnet';
parent + {
  helm+: { common+: { global+: {
    clusterGroup: 'tools',
    region: 'europe-dc1',
  } } },
  apps: parent.apps + [
    lib.grafanaOperator('grafana-operator', 'grafana-operator', 'grafana', true),
    lib.app('knot', 'knot', 'knot'),
  ],
}
