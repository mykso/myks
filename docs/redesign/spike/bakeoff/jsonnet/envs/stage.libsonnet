// LEAF env: tools-stage-eu-dc1. Sets identity, assembles leaf apps, removes
// bootstrap, derives env-scope flags, resolves all cross-app introspection
// (L4a/b/c) and the L7 computed fields in one eval scope.
local lib = import '../lib.libsonnet';
local parent = import 'tools.libsonnet';

// --- env-scope overrides (apps stay a list while inheriting) ---
local lvl = parent + {
  id: 'tools-stage-eu-dc1',
  helm+: { common+: { global+: { tier: 'stage', octet: 20 } } },
};
local g = lvl.helm.common.global;

// L7 derived identity. region must be known — assert stands in for the schema check.
assert std.objectHas(lib.regionShort, g.region) : "region '%s' missing from regionShort map" % g.region;
local clusterFullName = '%s-%s-%s' % [g.clusterGroup, g.tier, lib.regionShort[g.region]];
local clusterAddress = '10.0.0.%d' % g.octet;

// --- leaf apps: instantiate prototypes + per-app overrides ---
local karmaBase = lib.karma('karma', 'karma', 'karma', true, {
  replicas: 2,  // L1: beats prototype default 1
  config: {
    ui: { refresh: '30s' },
    alertmanager: [{ name: 'central', uri: 'http://10.0.0.1:9093' }],
  },
});
local cf = lib.centralForwarder('central-forwarder', 'central-forwarder', 'monitoring', true,
                                lib.cf(mode='vm-agent', enabled=true, shards=4, remoteWriteUrl='http://10.0.0.1:8427/api/v1/write'));
local arc = lib.arc('arc', 'arc', 'arc-system');
local alert = lib.alertmanager('alertmanager', 'alertmanager', 'monitoring', true);
local dash = lib.app('karma-dashboards', 'dashboards');  // namespace set via L4a below

// --- roster: drop bootstrap (remove-by-key), append leaf apps ---
local appList =
  std.filter(function(a) a.proto != 'bootstrap', lvl.apps)
  + [karmaBase, cf, arc, alert, dash];

// --- env-scope derivation ---
local runsGO = std.length(std.filter(function(a) a.proto == 'grafana-operator', appList)) > 0;  // L4c

// --- karma computed helm values (depends on env clusterFullName) ---
local karma = karmaBase + {
  helm: { values: {
    replicaCount: karmaBase.replicas,
    image: karmaBase.image,
    external_url: 'https://karma.%s.example.com' % clusterFullName,
  } },
};

// --- cross-app introspection, one eval scope ---
local byName = { [a.name]: a for a in appList };
// L4b: routes over monitored apps, name-sorted, alertmanager excludes itself.
// Jsonnet std.sort HAS a key fn (unlike KCL) — one call, no map-back.
local monitored = std.sort(
  std.filter(function(a) a.monitored && a.name != 'alertmanager', appList),
  function(a) a.name,
);
local routes = [{ receiver: a.name, namespace: a.namespace } for a in monitored];

// Apps whose final config layers cross-app / computed values onto the base.
local patched = {
  karma: karma,
  'karma-dashboards': dash + { namespace: byName.karma.namespace },  // L4a
  alertmanager: alert + { routes: routes },                          // L4b
  'central-forwarder': cf + { grafanaIntegration: runsGO },          // L4c
};
local apps = { [a.name]: (if std.objectHas(patched, a.name) then patched[a.name] else a) for a in appList };

{
  id: lvl.id,
  kubernetesDistribution: lvl.kubernetesDistribution,
  rancherEnabled: lvl.rancherEnabled,
  runsGrafanaOperator: runsGO,
  helm: lvl.helm + { common+: { global+: {
    clusterFullName: clusterFullName,
    clusterAddress: clusterAddress,
  } } },
  argocd: lvl.argocd,
  applications: apps,
}
