import myks

import envs

env = myks.finalize(envs.env | {
    id = "kcl-dev"
    applications.hello.application.greeting = "hello from dev"
    applications.hello.helm.namespace = "dev-hello"
})
