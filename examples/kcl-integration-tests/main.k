# Root evaluation: emits the frozen resolved tree consumed by myks.
# Environments are discovered from this output only — no filesystem walk.
import myks

import envs.dev

myksSchemaVersion = myks.SCHEMA_VERSION
environments = {
    dev = dev.env
}
