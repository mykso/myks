# Leaf finalizer: the single place leaf invariants are validated.
# Derivations and checks that would fire on intermediate levels belong here,
# not in schema `check` blocks (KCL re-evaluates those on every override).
finalize = lambda env: Environment -> Environment {
    # truthiness covers Undefined (attr never set), None and ""
    assert env.id, "a leaf environment must set a non-empty id"
    env
}
