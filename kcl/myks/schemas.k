schema Environment:
    """One node of the environment inheritance tree.

    Intermediate levels leave `id` unset; a leaf sets it and passes through
    `finalize`, which validates leaf invariants. No `check` block here:
    validation must not fire on intermediate (not-yet-finalized) instances.

    Extra keys (e.g. a `global` bag of shared values) are allowed and flow
    into the frozen tree; the engine reads only `id`, `argocd` and
    `applications`.
    """
    id?: str
    argocd?: {str:any}
    applications?: {str:App}
    [...str]: any

schema App:
    """Resolved configuration of one Application; the base for prototype schemas.

    `proto` selects the prototype (sync/render sources); it defaults to the
    application's key in `applications` when unset. All other keys are the
    app's resolved values, passed to the render pipeline as ytt data values.
    Prototype packages subclass this to type their own values.
    """
    proto?: str
    argocd?: {str:any}
    [...str]: any
