"""arc — L2 self-ref namespace, static helm = plain YAML. Sync config (_vendir) lives in
vendir.k, the sibling file in this same package, and is layered in here."""

import lib.prototypes as p

app = p.Arc {name = "arc", proto = "arc", namespace = "arc-system", vendir = _vendir}
