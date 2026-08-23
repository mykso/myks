# Root evaluation: emits the frozen resolved tree consumed by myks.
# Environments are discovered from this output only — no filesystem walk.
import envs.dev
import envs.prod

myksSchemaVersion = "0.1.0"
environments = {
    dev = dev.env
    prod = prod.env
}
