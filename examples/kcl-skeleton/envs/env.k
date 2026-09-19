# Base level: global defaults and the shared application roster.
# Leaf environments import this package and patch it.
import myks

env = myks.Environment {
    argocd.enabled = False
    applications.hello = myks.App {
        proto = "hello"
        argocd.enabled = False
        application.greeting = "hello"
    }
}
