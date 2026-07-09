# Bake-off fixture — concrete scenario matrix

One environment, `tools-stage-eu-dc1`, evaluated as a **whole tree** (ADR 0003): root → on-prem
→ tools → stage, every app resolved in one pass. `golden.yaml` is the expected resolved tree;
each candidate language must reproduce it (canonicalized via `yq -P 'sort_keys(..)'`).

This is the concrete form of the matrix in `../../requirements.md` §"Fixture scenario matrix".

## Inheritance tree (env levels)

| Level | Sets / overrides | Requirement |
|---|---|---|
| root | `kubernetesDistribution: gke`, `rancherEnabled: false`, apps `[bootstrap, cilium, namespaces]`, argocd syncOptions (`CreateNamespace: null` → dropped) | L1, L7 |
| on-prem (group) | `kubernetesDistribution: rke2`, `rancherEnabled: true`, append `[goldpinger, cluster-autoscaler]` | L1 override + append |
| tools (tier) | `clusterGroup: tools`, `region: europe-dc1`, append `[grafana-operator, knot]` | L1 |
| stage (leaf) | `id`, `tier: stage`, `octet: 20`, append `[karma, central-forwarder, arc, alertmanager, karma-dashboards]`, **remove bootstrap** | L1 append+remove |

Derived at env scope (ytt cannot — hand-set today):
- `runsGrafanaOperator: true` ← grafana-operator present in roster (L4c)
- `clusterFullName: tools-stage-eu-dc1` ← `{clusterGroup}-{tier}-{regionShort[region]}` (L7 computed)
- `clusterAddress: 10.0.0.20` ← `10.0.0.{octet}` (L7 computed)

## Apps and what each exercises

| App | proto | namespace | Exercises |
|---|---|---|---|
| `karma` | karma | karma | L1 leaf override (`replicas 1→2`), L7 typed defaults (`port 8080`), L6 any-passthrough `config`, **computed helm values** |
| `central-forwarder` | central-forwarder | monitoring | L7 dense validation (one_of/min/nullable/conditional), **L3** vendir source derived from `enabled` flag, **L4c** grafana dashboards iff grafana-operator present |
| `grafana-operator` | grafana-operator | grafana | presence drives L4c |
| `arc` | arc | arc-system | **L2** self-ref (`arc-system` → controller+runner ns), multi-chart vendir, **static helm = plain YAML** |
| `karma-dashboards` | dashboards | = `karma`'s ns | **L4a** B targets A's resolved namespace |
| `alertmanager` | alertmanager | monitoring | **L4b** routes = comprehension over every monitored app's namespace |
| `cilium`,`namespaces`,`goldpinger`,`cluster-autoscaler`,`knot` | (infra) | various | roster filler; carry `namespace`+`monitored` so L4b reads them |

`monitored: bool` is the L4b metadata mark every app carries. Monitored: goldpinger, grafana-operator,
karma, central-forwarder (→ 4 alertmanager routes, sorted by name; alertmanager excludes itself).

## Helm dual-case (requirements §"Helm values")

- **static** — `arc`: helm values are plain-YAML passthrough (copied from chart docs, unbeatable as YAML).
- **computed** — `karma`: helm values derived in-language from resolved config (`replicaCount`, `external_url`).

## Gate

`<lang>` driver emits the resolved tree → `yq -P 'sort_keys(..)'` both sides → `diff` → PASS.
golden.yaml is the independent oracle (design intent); ytt cannot express L2/L3/L4, so it is hand-authored.
