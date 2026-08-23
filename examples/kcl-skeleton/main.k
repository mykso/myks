# Root evaluation: emits the frozen resolved tree consumed by myks.
# Environments are discovered from this output only — no filesystem walk.
import myks

import envs.dev
import envs.prod

myksSchemaVersion = myks.SCHEMA_VERSION
environments = {
    dev = dev.env
    prod = prod.env
}
