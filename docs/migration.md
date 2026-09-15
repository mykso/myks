# Migrating to the KCL configuration layer

`myks migrate` converts a legacy ytt data-values repository into a KCL configuration tree
(the [config-layer redesign](redesign/design.md)). The conversion is a **machine seed plus a
hand-finish**: the converter produces a tree that renders byte-identically to the legacy
setup, and this guide covers turning that seed into idiomatic KCL.

## What the converter does

Running `myks migrate` in a legacy repository writes:

- `kcl.mod` — the module manifest, depending on the myks schema package. Its presence
  switches myks to the KCL path; the legacy data-values files are ignored from then on.
- `main.k` — the root evaluation emitting the frozen resolved tree; it imports every
  environment.
- `prototypes/<name>/proto.k` — one base schema per prototype, subclassing `myks.App` with
  the prototype's `app-data*` values as attribute defaults. Applications instantiate it
  instead of repeating those values ([ADR 0004](adr/0004-pure-language-config-surface.md)).
  A prototype directory whose name is not a valid KCL identifier is renamed to make it one;
  see below.
- `envs/**/*.k` — the files of each environment-tree level, mirroring the legacy layout of
  `env-data.yaml` plus one directory per application. A KCL package is a directory, so the
  files of one level share a namespace and cross-reference each other directly:
  - `env.k` — the level's converted `env-data.*.yaml` values, the import of the parent
    level, and the wiring (`id`, `myks.finalize` on a leaf). Always written.
  - `app-<name>.k` — everything this level says about one application: its declaration
    (with the values from `_proto/` and `_apps/` files, and from `prototypes/*/app-data*`
    when the prototype has no base schema, merged in) or its override of a declaration
    above, plus the values frozen from the legacy-resolved output. One file per application
    per level, the KCL counterpart of the legacy `_apps/<name>/` directory.
  - `patch.k` — `_patch`: environment values frozen from the legacy-resolved output, see
    below. Written only when the level needs one.

A data-values file that is a schema document (`#@data/values-schema`) is resolved by ytt
itself (`ytt --data-values-schema-inspect`), so its converted defaults carry the schema
semantics plain YAML parsing cannot see: a schema array declares only the type of its items
and defaults to `[]` unless `#@schema/default` says otherwise, `#@schema/default` wins over
the written value, and a `#@schema/nullable` key defaults to null. Its `#@schema/validation`
constraints feed the generated prototype schema's type and `check:` block where ytt reports
them in its OpenAPI output (`min_len`, `max_len`, `min`, `max`, `one_of`; see "Tighten the
prototype schemas" below) — any other validation, a custom rule or a keyword argument ytt does
not report, is not carried over; the converter warns naming each.

A file translates as plain YAML otherwise. It is skipped only when it contains **ytt
computation**: a directive with code after it (`#@ load(...)`, `key: #@ expr`), an overlay
directive that rewrites values instead of merging them (`#@overlay/remove`,
`#@overlay/replace`, `#@overlay/append`, `#@overlay/insert`), or a schema document ytt cannot
inspect standalone. A skipped file's *resolved* values are frozen as literals at the leaf
instead, marked with a `TODO(myks migrate)` comment: application values in that application's
file, environment values in `patch.k`. The result still renders identically; the literals are
yours to replace with real KCL derivations.

The application files unify into one accumulator, `_apps`, which `env.k` folds into
`applications`:

```kcl
# envs/shop/prod/app-forwarder.k
import myks as m
import prototypes.forwarder

_apps: m.Apps {
    forwarder = forwarder.Forwarder {
        application: {logLevel = "debug"}
    }
}
```

Adding an application is adding a file — nothing to register elsewhere. Within a level the
later block wins, which is how the frozen values override the declaration above them in the
same file. Splitting a level further is free the same way: any `.k` file you add next to
`env.k` joins the package, so a level's own schemas can live in a file of their own.

The legacy files are left in place so the conversion is easy to inspect and revert
(`git checkout` / delete `kcl.mod`, `main.k` and the generated level files).

## Step by step

1. **Prerequisites.** Directory names under `envs/` become KCL package names and must be
   valid identifiers (letters, digits, underscores): `central_forwarder`, not
   `central-forwarder`. Rename first if needed — the converter refuses otherwise. The
   environments base dir itself must not be an environment.

   Prototype directory names are subject to the same rule, but the converter handles them
   for you: `prototypes/cert-manager` becomes `prototypes/cert_manager`, together with the
   `_proto/cert-manager/` override directories of every environment level. **Application
   names are not affected** — they come from the legacy roster and stay the keys of the
   generated `applications` dict, so every rendered path stays where it is.

   Each legacy name is left behind as a symlink to the new directory, which keeps the legacy
   sources resolvable: `myks migrate --force` can re-read them, and you can still render the
   legacy tree (delete `kcl.mod`) to compare. Delete the symlinks once you drop the legacy
   data-values files.

   One thing does change: `myks.context.prototype`, which ytt templates can read, now carries
   the new directory name. Grep for it before converting; the gate in step 3 reports any
   rendered-output difference it causes.

   A name no rename can fix — one starting with a digit, a KCL keyword, or `m`/`parent`
   (which the generated level files already bind) — gets no base schema, and the converter
   warns.
2. **Convert:**

   ```sh
   myks migrate
   ```

   By default `kcl.mod` pins `oci://ghcr.io/mykso/myks` at the schema version this myks
   build supports. Use `--schema-package` to point at a fork or a local path.

   The converter never overwrites by accident: if any of `kcl.mod`, `main.k` or a generated
   level file already exists, it refuses and names the files. Re-run with `--force` to read the legacy
   files again and overwrite the generated ones — hand-written KCL is lost, so commit the
   seed before iterating on it.
3. **Run the gate.** The migration gate is byte-identical rendered output:

   ```sh
   git add -A && git commit -m 'wip: myks migrate seed'
   rm -rf rendered && myks render ALL ALL
   git diff --stat rendered/
   ```

   An empty diff means the conversion is faithful. Investigate any difference before going
   further — the converter's warnings (printed at the end of the run) list the known causes.
4. **Hand-finish** (see below), re-running the gate after each step.
5. **Delete the legacy data-values files** (`env-data.*.yaml`, `app-data.*.yaml`) once the
   gate is clean. All other files — prototypes, `vendir/`, `helm/`, `ytt/` overlays,
   `lib/` — stay: only data-values authoring moved to KCL.

## Hand-finish

### Replace frozen literals with derivations

Every frozen block is a value that used to be computed by ytt. Move it to where it belongs
and express the computation in KCL. Typical example — an app value derived from the
environment id:

```kcl
# seed (frozen literal in envs/dev/app-argocd-tests.k):
_apps: m.Apps {
    "argocd-tests": {application: {envId = "mykso-dev"}}
}
```

The files of a level share a namespace, so the application file reads the id bound by its
`env.k` — the derivation stays next to the value it feeds:

```kcl
# envs/dev/env.k
_id = "mykso-dev"
env = m.finalize(envs.env | {
    id = _id
    applications: {k: v for k, v in _apps}
})

# envs/dev/app-argocd-tests.k — one block, no frozen literal left
_apps: m.Apps {
    "argocd-tests": {application: {envId = _id}}
}
```

Delete the frozen block, and `patch.k` once its `_patch` is empty.

### Tighten the prototype schemas

When a prototype's `app-data*` file is a schema document ytt could inspect, `proto.k` already
carries what that schema said. A structured object value — one the ytt schema describes with
properties — becomes a KCL schema of its own, so every field keeps its name, its type (`str`,
`int`, `float`, `bool`, `{str:any}`, `[any]`, or `any` for `#@schema/type any=True`) and its
default, at any depth:

```kcl
schema Webapp(m.App):
    [...str]: any
    proto: str = "webapp"
    application?: WebappApplication = WebappApplication {}

schema WebappApplication:
    [...str]: any
    containerPort?: int = 80
    image?: str
    ingress?: bool = True

    check:
        len(image) >= 1 if image != Undefined, "application.image must be at least 1 long"
```

Every generated schema keeps an index signature, so an application may still set keys the
prototype never declared, exactly as ytt data values allowed. A value the ytt schema left
free-form stays a `{str:any}` literal default.

`#@schema/validation` bounds become `check:` items in the schema that owns the field. Reaching
into a free-form bag they are guarded by the keys on the way, so a check never fails on an
application that replaced the enclosing scope wholesale. A custom rule or any other validation
keyword argument the migration warned about is not in that block; restate it there by hand.

A KCL check runs where the schema is instantiated, that is where the application is declared,
and again on every override of that instance, while ytt validated the final data values of a
render once. So a value the prototype validates without supplying a satisfying default —
`min_len=1` on an empty string, which is how a ytt prototype demands a value — is declared
**without a default** (`image?: str` above) and its check guarded against the absence. The
value is then absent until some level sets it, and validated from that level down. What this
does not catch is an application that never sets it at all: KCL cannot express "required at
the leaf" in a schema whose instances are also the intermediate levels. If a value must be
present, assert it in the leaf finalizer instead.

One consequence to watch during migration: the value no longer defaults to the empty string,
so an application that legacy rendering resolved to `""` gets that empty string frozen as a
leaf patch — and then fails the check. That is a real invalid configuration, surfaced; fix it
by setting the value.

Without an inspected schema to draw on, the generated `proto.k` is a single loose schema:
containers keep their kind (`{str:any}`, `[any]`) so a `key: {...}` union merges into the
default instead of replacing it, and scalars are `any` so an application may override with a
different type, as plain ytt data values allowed. Splitting the bags out by hand is where the
schema starts catching mistakes:

```kcl
# seed:
schema Forwarder(m.App):
    proto: str = "forwarder"
    application?: {str:any} = {logLevel = "info"}

# hand-finished:
schema Forwarder(m.App):
    proto: str = "forwarder"
    application: ForwarderApplication = ForwarderApplication {}

schema ForwarderApplication:
    logLevel: str = "info"
```

A prototype the converter skipped (no base schema) has its values inlined into every
application declaration using it — that duplication is the signal to rename the directory
and write the schema by hand.

### Collapse the per-application files you do not need

One file per application per level is the seed's default, and the layout to keep for
anything with real configuration. A level whose applications are one-liners reads better
collapsed: any `.k` file of the level may declare several of them, so merge them into a
single `apps.k` and delete the rest — the accumulator is what the level folds in, not the
file names ([design](redesign/design.md#tree-layout)).

## Known limitations

The converter warns about each of these when it detects them:

- **`environment.*` extras.** The engine owns only `environment.id` and
  `environment.applications`; it regenerates both from the tree. Other keys of the scope are
  ordinary data and are carried over, so templates reading `data.values.environment.<key>`
  keep working.
- **Mapping key order.** KCL emits mapping keys sorted, while ytt kept file order. Values are
  unchanged, but a template that dumps a whole data-values subtree (`yaml.encode`, a Helm
  values file, a ConfigMap body) renders its keys in a different order — and anything hashing
  that text (a `checksum/config` annotation) changes with it.
- **Array semantics.** ytt *appends* data-values arrays onto schema defaults, while KCL
  dict union *replaces* lists. The converter simulates the real engine seam, so seeded
  trees are faithful — but a frozen array literal in a `_patch` can double up after
  hand-edits. Trust the gate.
- **Merge-order skew.** Legacy merges all `_proto` files (all levels) before all `_apps`
  files; the converted tree merges both per level. Values differing because of this are
  corrected by the leaf patches.
- **Arrays in custom scopes.** Custom (non-engine) scopes travel through a generated ytt
  schema document, which cannot carry multi-element arrays. Same limitation as legacy
  app-data schema files.

## Legacy support

The previous major release stays on a maintenance branch: bugfixes and trivial features on
demand for 12 months from the first stable KCL-mode release
([ADR 0005](adr/0005-go-host-evolve-in-place.md)).
