---
status: accepted
---

# A prototype's base schema lives in its own directory as `prototypes/<name>/proto.k`

ADR 0004 settled that apps import their prototype. This decides *where* the prototype's schema
lives: **inside the prototype directory**, as a package (`prototypes/<name>/proto.k`) that
subclasses `myks.App` and carries the prototype's defaults as attribute defaults. An application
declaration instantiates it and overrides only what it changes.

The KCL module root is the repo root, so `prototypes/<name>/` is already a valid package path;
no `kcl.mod` entry and no engine change are needed to import it.

## Why not somewhere else

A KCL import path is a sequence of **identifiers**, and a package is a directory — so a prototype
directory named `cert-manager` cannot be imported. Three alternatives were tested against
`kcl` v0.12.8 and rejected:

- **Quoted or escaped path segments** (`import prototypes."cert-manager"`, backticks) —
  `error[E1001]: InvalidSyntax`. No escape hatch exists.
- **A `kcl.mod` path-dependency alias** (`certmanager = { path = "prototypes/cert-manager" }`) —
  works, but only if the prototype directory also carries its own `kcl.mod`; without it `kcl`
  fails with a Go stack overflow rather than a readable error. It costs a manifest file per
  prototype plus a root dependency entry, and hand-added prototypes fail obscurely.
- **A flat `prototypes/<name>.k` package** — all schemas in one `prototypes` package, importable
  regardless of directory names (subdirectories are not compiled unless imported, so dashed
  prototype directories are harmless). Zero renames, but the schema stops being part of the
  prototype: a prototype directory is no longer the self-contained, copyable unit it is today.

Co-location won because a prototype should ship its own defaults the way it ships its `vendir/`,
`helm/` and `ytt/` directories.

## Consequences

- A prototype directory must be a valid KCL identifier — and not `m` or `parent`, which a
  generated `env.k` already binds — to own a base schema. This extends ADR 0004's
  identifier-safe naming rule from environments to prototypes.
- The rule is **not** enforced: a prototype that does not qualify gets no base schema, its
  defaults are inlined into every declaration as before, and `myks migrate` warns with the
  rename to perform. Existing repositories keep converting without a forced rename.
- Renaming a prototype directory changes the value of `myks.context.prototype`, which ytt
  templates can read. The byte-identical gate covers it; a roster entry with an explicit
  `name:` keeps the application name, and therefore every rendered path, unchanged.
- The generated schema is loose by design (`{str:any}` / `[any]` for containers, `any` for
  scalars) so that it reproduces ytt data-values semantics; tightening the types is a
  hand-finish step, and it is where the type checking ADR 0004 promised starts paying off.
