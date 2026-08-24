# Root level: shared defaults and the application roster with root-level values.
# This is the KCL port of the legacy fixture's envs/env-data.ytt.yaml plus the
# app-data.ytt.yaml files of prototypes/, envs/_proto/ and envs/_apps/.
import myks as m

env = m.Environment {
    argocd = {
        namespace = "system-argocd"
        app = {
            prefix = "app-"
            # Kept for parity with the legacy fixture; ytt ignores an empty array in a
            # data-values document, so the default finalizer stays in the rendered output.
            finalizers = []
            source = {
                plugin = {name = "argocd-vault-plugin-v1.0.0"}
                # Fixed config to run tests successfully in pipeline
                targetRevision = "main"
                repoURL = "git@github.com:mykso/myks.git"
            }
        }
        project = {prefix = "env-"}
    }
    # Fixed git config to run tests successfully in pipeline.
    myks = {
        gitRepoBranch = "main"
        gitRepoUrl = "git@github.com:mykso/myks.git"
    }
    applications = {
        "argocd-tests" = m.App {
            proto = "ytt-render-test"
            application = {
                baseValue = True
                protoOverwriteValue = True
            }
        }
        "helm-installation" = m.App {
            proto = "helm-render-test"
            application = {
                baseValue = True
                protoOverwriteValue = True
                envGroupValue = True
            }
        }
        "per-chart-override" = m.App {
            helm.charts = [
                {name = "render-test-1", releaseName = "overridden-release-name-1"}
                {name = "render-test-2", releaseName = "overridden-release-name-2"}
            ]
        }
        "multiple-sources" = m.App {}
        "multiple-contents-sources" = m.App {}
        "ytt-installation" = m.App {
            proto = "ytt-render-test"
            application = {
                baseValue = True
                protoOverwriteValue = True
                envGroupValue = True
            }
        }
        "static-test" = m.App {}
        "helm-myks-context" = m.App {}
        "lib-test" = m.App {}
    }
}
