import envs

env = envs.env | {
    id = "kcl-prod"
    applications.hello.helm.namespace = "prod-hello"
}
