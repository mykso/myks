import myks

import envs

env = myks.finalize(envs.env | {
    id = "kcl-prod"
    applications.hello.helm.namespace = "prod-hello"
})
