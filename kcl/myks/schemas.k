schema Environment:
    """One node of the environment inheritance tree.

    Intermediate levels leave `id` unset; a leaf sets it and passes through
    `finalize`, which validates leaf invariants. No `check` block here:
    validation must not fire on intermediate (not-yet-finalized) instances.

    Extra keys (e.g. a `global` bag of shared values) are allowed and flow
    into the frozen tree; the engine reads only `id`, `argocd` and
    `applications`. An `environment` key is merged under the data-values
    scope of the same name, beneath the engine-owned `id` and `applications`.
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

    Every key reaches the data values, underscore-prefixed ones included: the
    engine evaluates with `show_hidden`, so KCL's private-member convention
    does not apply here — there is no place to hide a helper value.
    """
    proto?: str
    argocd?: {str:any}
    [...str]: any

schema Apps:
    """Accumulator for the per-application files of one environment level.

    A KCL package is a directory, so every `.k` file of a level shares one
    namespace and a schema-typed declaration of the same name unifies across
    them. Each application file declares `_apps: Apps { <name> = ... }`, and
    the level's `env.k` folds the unified result into `applications` — so
    everything one level says about an application lives in one file.

    The fold is a dict comprehension (`{k: v for k, v in _apps}`): a schema
    instance on the right of `|` or `:` replaces instead of merging, which
    would drop the values inherited from the parent level.
    """
    [...str]: App
