"""arc sync config — own file (same package as app.k, shares scope). Demonstrates the
per-app layout splitting render config from sync config the way real myks does. This
overrides the prototype default vendir with the app-specific multi-chart source set."""

_vendir = {
    sources = {
        controller = {helmChart = {name = "gha-runner-scale-set-controller", url = "https://actions.github.io/actions-runner-controller-charts", version = "0.9.3"}}
        "runner-set" = {helmChart = {name = "gha-runner-scale-set", url = "https://actions.github.io/actions-runner-controller-charts", version = "0.9.3"}}
    }
}
