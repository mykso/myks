# Leaf environment: identity plus dev-level application overrides.
# Note the `:` (merge) on dict attributes — `=` would replace the inherited dict wholesale.
import myks as m

import envs

_id = "mykso-dev"

env = m.finalize(envs.env | {
    id = _id
    applications: {
        "argocd-tests": {
            # envId is derived from the environment id — in the legacy fixture this was a
            # ytt load of the environment data lib in the prototype's app-data file.
            application: {envId = _id}
            argocd: {app: {source: {plugin = None}}}
        }
        "helm-installation": {application: {appValue = True}}
        "ytt-installation": {application: {envId = _id, appValue = True}}
    }
})
