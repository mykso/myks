"""alertmanager — L4b routes are a comprehension over the monitored roster,
computed at the env scope (needs every app), layered onto this base instance."""

import lib.prototypes as p

app = p.Alertmanager {name = "alertmanager", proto = "alertmanager", namespace = "monitoring", monitored = True}
