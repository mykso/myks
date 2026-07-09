// Shared base: env-scope constants, the App base, prototype constructors,
// and the one helper Jsonnet's stdlib lacks (list append on merge).
{
  // region -> short code. Local-ish: kept out of emitted output (it's not on `env`).
  regionShort:: {
    'europe-dc1': 'eu-dc1',
    'europe-west4': 'eu-w4',
    'us-central1': 'us-c1',
    'asia-southeast1': 'as-se1',
  },

  // The only merge helper Jsonnet needs: `+` deep-merges objects but *replaces*
  // arrays. Env levels append to `apps`, so a parent passes its list through
  // `appendApps` and a child level concatenates. Cost: one explicit `+ [...]`
  // per appending level vs KCL's native `apps += [...]`.
  appendApps(base, extra):: base + extra,

  // Base application. namespace defaults to name; monitored is the L4b mark.
  // No static typing — `app()` is a constructor with an assert standing in for
  // a schema. overrides is merged last (last-wins, like a leaf override).
  app(name, proto, namespace=null, monitored=false, overrides={})::
    {
      name: name,
      proto: proto,
      namespace: if namespace == null then name else namespace,
      monitored: monitored,
    } + overrides,

  // --- prototypes (typed defaults an app extends) ---

  // karma — L7-ish defaults + L6 any-passthrough `config`. helm computed at leaf.
  karma(name, proto, namespace, monitored, overrides={})::
    $.app(name, proto, namespace, monitored, {
      port: 8080,
      image: 'registry.example.com/ghcr-io-mirror/prymitive/karma:v0.131',
      replicas: 1,
    } + overrides),

  // central-forwarder — L7 dense validation via assert; L3 vendir derived from `enabled`.
  cf(mode='prometheus-agent', enabled=false, shards=2, alertmanagerVersion='v0.27.0', remoteWriteUrl='')::
    assert std.member(['prometheus', 'prometheus-agent', 'vm-agent'], mode) :
      "cf.mode must be one of prometheus|prometheus-agent|vm-agent, got '%s'" % mode;
    assert shards >= 1 : 'cf.shards must be >= 1, got %d' % shards;
    assert !enabled || std.length(remoteWriteUrl) > 0 : 'set remoteWrite.url when enabling the forwarder';
    {
      mode: mode,
      enabled: enabled,
      shards: shards,
      alertmanagerVersion: alertmanagerVersion,
      remoteWrite: { url: remoteWriteUrl },
    },

  centralForwarder(name, proto, namespace, monitored, cf)::
    $.app(name, proto, namespace, monitored, {
      centralForwarder: cf,
      grafanaIntegration: false,  // set from env runsGrafanaOperator at leaf (L4c)
      // L3: one source of truth — vm-agent source appears iff enabled.
      vendir: {
        sources: {
          namespace: { helmChart: { name: 'namespace', url: 'https://charts.example.com', version: '3.0.0' } },
        } + (if cf.enabled then {
               'vm-agent': { helmChart: { name: 'victoria-metrics-agent', url: 'https://victoriametrics.github.io/helm-charts', version: '0.14.0' } },
             } else {}),
      },
    }),

  grafanaOperator(name, proto, namespace, monitored)::
    $.app(name, proto, namespace, monitored, {
      image: 'ghcr.io/grafana/grafana-operator:v5.20.0',
    }),

  // arc — L2 self-ref: one namespace feeds controller+runner. Static helm = plain YAML.
  arc(name, proto, namespace)::
    $.app(name, proto, namespace, false, {
      controllerNamespace: namespace,
      runnerNamespace: namespace,
      helm: { values: { githubConfigUrl: 'https://github.com/myorg', minRunners: 1, maxRunners: 10 } },
      vendir: {
        sources: {
          controller: { helmChart: { name: 'gha-runner-scale-set-controller', url: 'https://actions.github.io/actions-runner-controller-charts', version: '0.9.3' } },
          'runner-set': { helmChart: { name: 'gha-runner-scale-set', url: 'https://actions.github.io/actions-runner-controller-charts', version: '0.9.3' } },
        },
      },
    }),

  alertmanager(name, proto, namespace, monitored)::
    $.app(name, proto, namespace, monitored, { routes: [] }),
}
