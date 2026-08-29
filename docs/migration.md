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
  Only prototypes whose directory name is a valid KCL identifier get one; see below.
- `envs/**/env.k` — one file per environment-tree level. Each level converts its
  `env-data.*.yaml` values, declares the applications rostered there (with the values from
  `_proto/` and `_apps/` files, and from `prototypes/*/app-data*` when the prototype has no
  base schema, merged in), and overrides applications declared above it.

Data-values files containing **ytt logic** (`#@ load(...)`, inline `#@` expressions,
`#@schema/default` annotations) cannot be translated. The converter skips them and instead
freezes their *resolved* values as literals in a leaf-level `_patch` block marked with a
`TODO(myks migrate)` comment. The result still renders identically; the literals are yours
to replace with real KCL derivations.

The legacy files are left in place so the conversion is easy to inspect and revert
(`git checkout` / delete `kcl.mod`, `main.k` and the `env.k` files).

## Step by step

1. **Prerequisites.** Directory names under `envs/` become KCL package names and must be
   valid identifiers (letters, digits, underscores): `central_forwarder`, not
   `central-forwarder`. Rename first if needed — the converter refuses otherwise. The
   environments base dir itself must not be an environment.

   Prototype directory names are subject to the same rule, but it is **optional**: a
   prototype whose name is not an identifier (or is `m` or `parent`, which a generated
   `env.k` already binds) simply gets no base schema, and the converter warns. To get one,
   rename the directory and update the roster entries that reference it. Give the entry an
   explicit `name:` to keep the application name — and every rendered path — unchanged:

   ```yaml
   - proto: cert_manager
     name: cert-manager
   ```

   Renaming also changes `myks.context.prototype`, which ytt templates can read; the gate
   in step 3 catches any rendered-output difference that causes.
2. **Convert:**

   ```sh
   myks migrate
   ```

   By default `kcl.mod` pins `oci://ghcr.io/mykso/myks` at the schema version this myks
   build supports. Use `--schema-package` to point at a fork or a local path.

   The converter never overwrites by accident: if any of `kcl.mod`, `main.k` or an `env.k`
   already exists, it refuses and names the files. Re-run with `--force` to read the legacy
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

Every `_patch` block is a value that used to be computed by ytt. Move it to where it
belongs and express the computation in KCL. Typical example — an app value derived from the
environment id:

```kcl
# seed (frozen literal in _patch):
"argocd-tests": {application: {envId = "mykso-dev"}}

# hand-finished (derivation at the leaf):
_id = "mykso-dev"
env = m.finalize(envs.env | {
    id = _id
    applications: {
        "argocd-tests": {application: {envId = _id}}
    }
})
```

Delete the `_patch` block when it is empty.

### Tighten the prototype schemas

The generated `proto.k` is deliberately loose: containers keep their kind (`{str:any}`,
`[any]`) so a `key: {...}` union merges into the default instead of replacing it, and
scalars are `any` so an application may override with a different type, as ytt data values
allowed. Typing the fields is where the schema starts catching mistakes:

```kcl
# seed:
schema Forwarder(m.App):
    proto: str = "forwarder"
    application?: {str:any} = {logLevel = "info"}

# hand-finished:
schema Forwarder(m.App):
    proto: str = "forwarder"
    application: Application = Application {}

schema Application:
    logLevel: str = "info"
```

A prototype the converter skipped (no base schema) has its values inlined into every
application declaration using it — that duplication is the signal to rename the directory
and write the schema by hand.

### Per-app opt-in layout

Flat declarations in `env.k` are the default. Move an application with substantial
configuration into its own package (`envs/<...>/<app_name>/app.k`) and import it — both
styles coexist ([design](redesign/design.md#tree-layout)).

## Known limitations

The converter warns about each of these when it detects them:

- **`environment.*` extras.** The engine owns the `environment` scope (id and application
  roster). Any other key under it in legacy env-data is dropped; move such values to
  another scope by hand.
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
