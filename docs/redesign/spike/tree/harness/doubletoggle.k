"""
Edge 5 — the DOUBLE-TOGGLE, killed.

Today (ytt): prototypes/central-forwarder/vendir/values.ytt.yaml hand-lists the
vm-agent source, and envs/.../_apps/central-forwarder enables the workload. Two
toggles for one decision, kept in sync BY HAND, because ytt data values cannot
self-reference (the vendir file may not read application.centralForwarder.enabled).

KCL: ONE source of truth. The vendir source set is DERIVED from the app flag.
Run with -D enabled=true|false to see the vm-agent source appear/disappear.

  kcl run doubletoggle.k -D enabled=true
"""

# NB: `-D enabled=true` is parsed by KCL as a BOOLEAN, not the string "true".
_enabled = option("enabled")

_namespace = {
    helmChart = {name = "namespace", url = "https://charts.example.com", version = "3.0.0"}
}
_vmAgent = {
    helmChart = {name = "victoria-metrics-agent", url = "https://victoriametrics.github.io/helm-charts", version = "0.14.0"}
}

prototype = {
    sources = {namespace = _namespace} | ({"vm-agent" = _vmAgent} if _enabled else {})
}
