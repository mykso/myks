---
title: Migrating to the KCL configuration layer
sidebar:
  order: 16
---

`myks migrate` converts a legacy ytt data-values repository into a KCL
configuration tree (the
[config-layer redesign](https://github.com/mykso/myks/blob/main/docs/redesign/design.md)).
The conversion is a **machine seed plus a hand-finish**: the converter produces
a tree that renders byte-identically to the legacy setup, and this guide covers
turning that seed into idiomatic KCL.
[The KCL converter](../../concepts/kcl-converter/) explains every file it
writes.

The legacy files are left in place so the conversion is easy to inspect and
revert (`git checkout` / delete `kcl.mod`, `main.k` and the generated level
files).

## Step by step

1. **Prerequisites.** Directory names under `envs/` become KCL package names and
   must be valid identifiers (letters, digits, underscores):
   `central_forwarder`, not `central-forwarder`. Rename first if needed — the
   converter refuses otherwise. The environments base dir itself must not be an
   environment.

   Prototype directory names are subject to the same rule, but the converter
   handles them for you: `prototypes/cert-manager` becomes
   `prototypes/cert_manager`, together with the `_proto/cert-manager/` override
   directories of every environment level. **Application names are not
   affected** — they come from the legacy roster and stay the keys of the
   generated `applications` dict, so every rendered path stays where it is.

   Each legacy name is left behind as a symlink to the new directory, which
   keeps the legacy sources resolvable: `myks migrate --force` can re-read them,
   and you can still render the legacy tree (delete `kcl.mod`) to compare.
   Delete the symlinks once you drop the legacy data-values files.

   One thing does change: `myks.context.prototype`, which ytt templates can
   read, now carries the new directory name. Grep for it before converting; the
   gate in step 3 reports any rendered-output difference it causes.

   A name no rename can fix — one starting with a digit, a KCL keyword, or
   `m`/`parent` (which the generated level files already bind) — gets no base
   schema, and the converter warns.

2. **Convert:**

   ```sh
   myks migrate
   ```

   By default `kcl.mod` pins `oci://ghcr.io/mykso/myks` at the schema version
   this myks build supports. Use `--schema-package` to point at a fork or a
   local path.

   The converter never overwrites by accident: if any of `kcl.mod`, `main.k` or
   a generated level file already exists, it refuses and names the files. Re-run
   with `--force` to read the legacy files again and overwrite the generated
   ones — hand-written KCL is lost, so commit the seed before iterating on it.

3. **Run the gate.** The migration gate is byte-identical rendered output:

   ```sh
   git add -A && git commit -m 'wip: myks migrate seed'
   rm -rf rendered && myks render ALL ALL
   git diff --stat rendered/
   ```

   An empty diff means the conversion is faithful. Investigate any difference
   before going further — the converter's warnings (printed at the end of the
   run) and its
   [known limitations](../../concepts/kcl-converter/#known-limitations) list the
   known causes.

4. **Hand-finish** (see below), re-running the gate after each step.
5. **Delete the legacy data-values files** (`env-data.*.yaml`,
   `app-data.*.yaml`) once the gate is clean. All other files — prototypes,
   `vendir/`, `helm/`, `ytt/` overlays, `lib/` — stay: only data-values
   authoring moved to KCL.

## Hand-finish

### Replace frozen literals with derivations

Every frozen block is a value that used to be computed by ytt and that the
converter could not carry over. Move it to where it belongs and express the
computation in KCL. Typical example — an app value derived from the environment
id, which the engine regenerates and `_level` therefore carries under `id`
rather than under `environment.id`:

```kcl
# seed (frozen literal in envs/dev/app-argocd-tests.k):
_apps: myks.Apps {
    "argocd-tests": {
        application.envId = "mykso-dev"
    }
}
```

The files of a level share a namespace, so the application file reads the
level's own environment data — the derivation stays next to the value it feeds:

```kcl
# envs/dev/app-argocd-tests.k — one block, no frozen literal left
_apps: myks.Apps {
    "argocd-tests": {
        application.envId = _level.id
    }
}
```

Delete the frozen block, and `patch.k` once its `_patch` is empty.

An array is frozen whole even where the converter translated the values inside
it: ytt resolved the array as one value, and its elements are literals the
source stated. Its elements state only what differs from their element schema,
which KCL instantiates them against. Shortening such a block further, to what
the level actually changes, is a hand-finish step the gate will check.

### Tighten the prototype schemas

When a prototype's `app-data*` file is a ytt schema, `proto.k` already carries
its types, defaults and validations (see
[Prototype schemas](../../concepts/kcl-converter/#prototype-schemas)). Without
an inspected schema to draw on, the generated `proto.k` is a single loose
schema: containers keep their kind (`{str:any}`, `[any]`) so a `key: {...}`
union merges into the default instead of replacing it, and scalars are `any` so
an application may override with a different type, as plain ytt data values
allowed. Splitting the bags out by hand is where the schema starts catching
mistakes:

```kcl
# seed:
schema Forwarder(myks.App):
    proto: str = "forwarder"
    application?: {str:any} = {logLevel = "info"}

# hand-finished:
schema Forwarder(myks.App):
    proto: str = "forwarder"
    application: ForwarderApplication = ForwarderApplication {}

schema ForwarderApplication:
    logLevel: str = "info"
```

A prototype the converter skipped (no base schema) has its values inlined into
every application declaration using it — that duplication is the signal to
rename the directory and write the schema by hand.

### Arrange the application files

The seed writes one file per application that the level configures, and gathers
the bare declarations into `apps.k`. Any `.k` file of the level may declare
applications, since the accumulator is what the level folds in, not the file
names
([design](https://github.com/mykso/myks/blob/main/docs/redesign/design.md#tree-layout)):
move an application out of `apps.k` once it gains values, or group related ones
as you see fit. Within one `_apps` block an application's key shadows a package
of the same name for the rest of the block: a declaration that reads such a
package goes before that key, or into a block of its own. Keep an application's
declaration and the overrides its own level states in one file: KCL unifies the
files of a level in file name order, and an `x: {...}` override in a file
sorting before the one declaring `x = X {...}` is dropped.

### Renovate

Renovate regex managers written for the legacy files match `#! renovate:` above
YAML's `image: x:1`. The generated KCL keeps the marker as `# renovate:` above
`image: str = "x:1"` (a schema default) or `image = "x:1"` (a level's value), so
those patterns no longer match. Worse, they keep matching the legacy files,
whose bumps no longer render anything, while the versions in the KCL tree never
move. The converter names every converted file that carries a marker.

Point the managers at the `.k` files in the same change that deletes the legacy
data-values files. Patterns covering the three shapes the converter writes:

```yaml
customManagers:
  - customType: regex
    managerFilePatterns: ["/\\.k$/"]
    matchStrings:
      # # renovate: datasource=docker versioning=semver-coerced
      # image: str = "ghcr.io/org/app:1.2.3"
      - '#!? renovate: datasource=(?<datasource>docker)(?:
        versioning=(?<versioning>\S+))?\s*\n\s*\w+\??(?::\s*\w+)?\s*=\s*"(?<depName>[^":@]+):(?<currentValue>[^"@]+)"'
      # # renovate: datasource=github-releases depName=traefik/traefik
      # version: str = "v3.7.13"
      - '#!? renovate: datasource=(?<datasource>\S+)
        depName=(?<depName>\S+)\s*\n\s*\w+\??(?::\s*\w+)?\s*=\s*"(?<currentValue>[^"]+)"'
      # # renovate: datasource=github-releases depName=helm/helm version=4.3.0
      - '#!? renovate: datasource=(?<datasource>\S+) depName=(?<depName>\S+)
        version=(?<currentValue>\S+)'
```

The optional `!` lets one pattern serve the legacy files as well while they
remain, so a version bump never leaves the two out of step.

## Legacy support

The previous major release stays on a maintenance branch: bugfixes and trivial
features on demand for 12 months from the first stable KCL-mode release
([ADR 0005](https://github.com/mykso/myks/blob/main/docs/adr/0005-go-host-evolve-in-place.md)).
