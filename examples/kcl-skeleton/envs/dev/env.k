import envs

env = envs.env | {
    id = "kcl-dev"
    applications.hello.application.greeting = "hello from dev"
    applications.hello.helm.namespace = "dev-hello"
}
