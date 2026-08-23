# Base level: global defaults and the shared application roster.
# Leaf environments import this package and patch it.
env = {
    argocd.enabled = False
    applications.hello = {
        proto = "hello"
        argocd.enabled = False
        application.greeting = "hello"
    }
}
