---
title: The KCL converter
sidebar:
  order: 4
---

`myks migrate` turns a legacy ytt data-values repository into a KCL
configuration tree that renders byte-identically. This page explains what it
writes and why; the [migration guide](../../guides/migration/) has the steps.

## What the converter does

Running `myks migrate` in a legacy repository writes:

- `kcl.mod` — the module manifest, named after the repository directory and
  depending on the myks schema package. Its presence switches myks to the KCL
  path; the legacy data-values files are ignored from then on.
- `main.k` — the entry point myks evaluates; every environment must be listed in
  its `environments`.
- `prototypes/<name>/proto.k` — one base schema per prototype, subclassing
  `myks.App` with the prototype's `app-data*` values as attribute defaults.
  Applications instantiate it instead of repeating those values
  ([ADR 0004](https://github.com/mykso/myks/blob/main/docs/adr/0004-pure-language-config-surface.md)).
  A prototype directory whose name is not a valid KCL identifier is renamed to
  make it one; see below.
- `envs/**/*.k` — the files of each environment-tree level, mirroring the legacy
  layout of `env-data.yaml` plus one directory per application. A KCL package is
  a directory, so the files of one level share a namespace and cross-reference
  each other directly:
  - `env.k` — the level's converted `env-data.*.yaml` values, the import of the
    parent level, and the wiring (`id`, `myks.with_apps`, `myks.finalize` on a
    leaf). Always written.
  - `app-<name>.k` — everything this level says about one application: its
    declaration (with the values from `_proto/` and `_apps/` files, and from
    `prototypes/*/app-data*` when the prototype has no base schema, merged in)
    or its override of a declaration above, plus the values frozen from the
    legacy-resolved output. One file per application per level, the KCL
    counterpart of the legacy `_apps/<name>/` directory.
  - `apps.k` — the applications the level only declares, with nothing else to
    say about them: one line each, so they share a file instead of getting one
    apiece.
  - `patch.k` — `_patch`: environment values frozen from the legacy-resolved
    output, see below. Written only when the level needs one.

A data-values file that is a schema document (`#@data/values-schema`) is
resolved by ytt itself (`ytt --data-values-schema-inspect`, with the repo's ytt
library directory on the path), so its converted defaults carry the schema
semantics plain YAML parsing cannot see: a schema array declares only the type
of its items and defaults to `[]` unless `#@schema/default` says otherwise,
`#@schema/default` wins over the written value, and a `#@schema/nullable` key
defaults to null. Its `#@schema/validation` constraints feed the generated
prototype schema's type and `check:` block: those ytt reports in its OpenAPI
output (`min_len`, `max_len`, `min`, `max`, `one_of`), plus `not_null`, which it
does not report and the converter reads from the annotation text (see
[Prototype schemas](#prototype-schemas)). Any other validation — a custom rule,
or a keyword argument with no KCL counterpart — is not carried over; the
converter warns naming each with its path.

A file translates as plain YAML otherwise. **ytt computation** — a directive
with code after it (`#@ load(...)`, `key: #@ expr`), an annotation that rewrites
values instead of merging them (`#@overlay/remove`, `#@overlay/replace`,
`#@overlay/append`, `#@overlay/insert`, `#@yaml/text-templated-strings`) — is
handled in the closest way that still renders identically:

- **The file resolves on its own.** Its Starlark reads nothing outside itself (a
  repo-local helper from the ytt library directory is fine), so ytt is asked for
  the answer and the computation is translated to KCL _where the file sits_ — a
  prototype's into its `proto.k`, a level's into that level's files (see
  "Translated ytt computation" below). What has no translation is converted as
  the literal ytt resolved, and the report names those paths.
- **The file needs the render context.** Its Starlark loads `@ytt:data` or
  `@myks:...`, so what it would answer standalone is not what it answers at
  render time. The file is then **split**: the values it states plainly are
  converted as usual, and only the computed ones are left out, to be frozen per
  leaf — or, where the computation reads the environment and KCL reproduces it
  at that leaf, translated there (see below). The same happens when a
  self-contained file fails to resolve.
- **Nothing is left after the split.** Every value in the file is computed, so
  the file is skipped whole.

Whatever the split leaves out has its _resolved_ value frozen as a literal at
the leaf, marked with a `TODO(myks migrate)` comment: application values in that
application's file, environment values in `patch.k`. The result still renders
identically; those literals are yours to replace with real KCL derivations. The
marker is left out where every frozen value is accounted for — translated (see
below), or stated plainly by the source next to the computed ones, as the other
entries of an array that ytt resolved whole are.

A value ytt resolved standalone carries no such comment, translated or not: the
file it came from reads nothing outside itself, so the literal states everything
its computation stated. The report names the untranslated ones as a readability
note, not as a task.

Within one directory the schema documents are merged before the plain
data-values documents, the way ytt resolves them — a schema's defaults never win
over a value file that sorts before it.

The application files unify into one accumulator, `_apps`, which `env.k` folds
into `applications` with `myks.with_apps(_level, _apps)`:

```kcl
# envs/shop/prod/app-forwarder.k
import myks
import prototypes.forwarder

_apps: myks.Apps {
    forwarder = forwarder.Forwarder {
        application.logLevel = "debug"
    }
}
```

A chain of single-key maps is written as one dotted key
(`storage.media.size = "1Gi"`), which merges into the defaults exactly as the
nested union would, and a dict of a few scalars stays on one line. Adding an
application is adding a file — nothing to register elsewhere. Within a level the
later block wins, which is how the frozen values override the declaration above
them in the same file. Frozen values with nothing left to do (no
`TODO(myks migrate)` marker) need no block of their own: they are written into
the declaration, or the override, the level states anyway. Splitting a level
further is free the same way: any `.k` file you add next to `env.k` joins the
package, so a level's own schemas can live in a file of their own. The shared
namespace cuts both ways: a module-level variable bound in two files of a level
is one variable, which every reader sees with the value bound last. The
converter prefixes a helper an application file binds with the application's
name (`_web_get_uri`) where another file of the level binds the same name
differently; keep to that when adding helpers by hand.

### Comments

What a data-values file wrote above one of its values travels with that value:
the note explaining why it is what it is, a tool's annotation such as
Renovate's. That holds inside a sequence element too, and blank lines between
entries stay, as does the order the file states its keys in. A ytt comment
marker (`#!`) means nothing outside a ytt template, so only its `#` is kept — a
pattern matching such a comment in the generated files has to look for `# ` and
for KCL's `key: type = value` or `key = value` rather than YAML's `key: value`.
The converter warns when a converted file carries a Renovate marker; see
[Renovate](../../guides/migration/#renovate). A carried comment that names a
legacy file (`app-data`, `env-data`, `*.ytt.yaml`, `_apps/`), or a prototype
directory by the name the converter renamed, is reported, since it stops being
true once the KCL tree replaces that file or the compatibility symlink is gone.

A `#@` directive is not a comment but code, and is never carried: what it did is
stated in KCL's own terms, or reported as not carried over.

The block a file ends with belongs to no value; the generated file ends with it
instead, so a marker tracking a version the file does not state is not lost with
the legacy file. A comment after a value, on its line, has no attribute of its
own to sit above and is left behind.

### Translated ytt computation

Starlark and KCL share their literals, operators and comprehensions, so a file
ytt resolves standalone is carried over as the derivation it was rather than the
value it produced:

| ytt                                                                                       | generated KCL                                              |
| ----------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| `#@ port = 8080` … `port: #@ port`                                                        | `_port = 8080` … `port: int = _port`                       |
| `purge_files_after: #@ 60 * 60 * 24`                                                      | `purge_files_after: int = 60 * 60 * 24`                    |
| `#@ hosts = [domain]` `#@ for n in nodes:` `#@   hosts.append(n + "." + domain)` `#@ end` | `_hosts = [_domain] + [n + "." + _domain for n in _nodes]` |
| `#@ load("secrets.star", "sops")` … `token: #@ sops("0", "api")`                          | `import lib` … `token: str = lib.sops("0", "api")`         |
| `#@ for/end reg in registries:` `- name: #@ reg`                                          | `[{name = reg} for reg in _registries]`                    |
| `#@yaml/text-templated-strings` `key: "(@= sops("0", "k") @)\n"`                          | `key = """\` … `${lib.sops("0", "k")}` … `"""`             |

The prelude's top-level assignments become module-level `_`-prefixed variables
of the generated file, pruned to the ones a derivation reads; an assignment
indented into the YAML body, as the list a `for/end` loop iterates usually is,
counts as one too. A loop whose body only appends to lists becomes a
comprehension over the same iterable, joined to the list's binding where nothing
in between depends on the order. Comments in the prelude come along, including
those between the elements of a list. A `for/end` loop over a sequence item
becomes a list comprehension when the item computes nothing but inline
expressions; a text template becomes a KCL string interpolating the same
expressions.

Every function of the repo's ytt library whose body is a single `return` is
translated to a KCL lambda in `<ytt-library-dir>/<file>.k`, imported as one
package by the files that call it. Only the lambdas a generated file calls are
written. The `.star` sources stay where they are: ytt templates still load them.

**Nothing is taken on trust.** Each file's derivations are evaluated by KCL —
its prelude, its imports, one variable per derived value — and compared against
what ytt resolved. A derivation KCL does not reproduce, or a file that does not
evaluate at all, falls back to the literal. What has no translation at all: a
ytt template function (a `def` with a YAML body), `#@ for`/`#@ end` inside the
YAML body, a keyword argument, Starlark's `%` string formatting, and
`@ytt:data`'s `data.values`.

#### Values an application derives from its environment

A file loading `@myks:data.lib.yaml` reads the data values of its environment,
which only a leaf has — it never resolves standalone, and its computed values
are frozen per leaf. Those values are translated too: `env_data` becomes
`_level`, the level's environment data, which `env.k` binds before folding the
applications in, so the level's application files can read it.

```kcl
# envs/alpha/app-traefik.k
_apps: myks.Apps {
    traefik = traefik.Traefik {
        application.tls: {
            baseDomains = _level.environment.hosts
            issuer = "cloudflare-zerossl"
        }
    }
}
```

The proof is per leaf, and so is the result: the translation is evaluated
against the environment data resolved for that leaf and compared with the value
frozen there. Where it reproduces it, the leaf states the derivation; where it
does not — the value differs below the level the file sits at — that leaf keeps
the literal. Nothing is hoisted above the leaf that proved it.

An **environment** value is never translated this way: `patch.k` is what
`_level` is built from, so a derivation reading `_level` inside it would be
circular.

## Prototype schemas

When a prototype's `app-data*` file is a schema document ytt could inspect,
`proto.k` already carries what that schema said. A structured object value — one
the ytt schema describes with properties — becomes a KCL schema of its own, so
every field keeps its name, its type (`str`, `int`, `float`, `bool`,
`{str:any}`, a list of its element type such as `[str]`, or `any` for
`#@schema/type any=True`) and its default, at any depth:

```kcl
schema Webapp(myks.App):
    proto: str = "webapp"
    application: Application = Application {}

schema Application:
    containerPort: int = 80
    image?: str
    ingress: bool = True

    check:
        len(image) >= 1 if image != Undefined, "application.image must not be empty"
```

An attribute the ytt schema types without allowing null is required
(`containerPort: int`): its default applies wherever no level sets it, and
setting it to `None` fails, as ytt failed. A nullable value, one typed `any`,
and one no schema document describes keep the `?` that lets them be `None`. The
generated schemas, and the checks in them, follow the order the source states
the fields in.

A schema is named after the last segment of the path to it — `Application`, not
`WebappApplication` — and lengthened with the segments above only where that
name is already taken (`ServerTls` next to `Tls`). Every prototype is a KCL
package of its own, so the short name is what call sites read:
`webapp.Application`.

An array whose ytt schema describes its element gets an element schema the same
way, named after the array in the singular (`ConfigItem` where the name reads as
no plural), and the attribute is typed `[Element]`. A `min_len` on the array,
and any validation on the element's keys, become checks there too:

```kcl
schema Application:
    clients: [Client] = []

schema Client:
    host: str
    port: int = 5001
    routes: [Route] = []

schema Route:
    path: str = "/"
```

The values a ytt schema writes into an array item are both the default of every
element and the example of its shape. Where every element the sources state — at
any level, computed or not — sets a scalar field, the example is dropped and the
field is required instead (`host` above): it changes nothing the tree renders,
and an element added later has to set the field too, rather than silently
becoming `"Arch"` or `"TZ"`. A field some element leaves out keeps its default.

KCL instantiates every element of such a list, so an application writing
`clients = [{host = "h"}]` still gets `port` and `routes` — which is what ytt
did by overlaying the stated array onto the schema's. This reaches as deep as
the ytt schema describes: an object or an array inside an element is typed too,
and its defaults are filled into what an application writes there.

A generated schema keeps an index signature (`[...str]: any`) only where the ytt
schema did not close the scope. A data-values schema is strict — it rejects a
key it does not declare, whichever file states it — so a scope it governed needs
none, and the generated schema catches the misspelled key ytt used to catch. The
engine scopes `argocd`, `helm`, `kbld`, `render` and `yttPkg` are declared on
`myks.App` itself, so every application may set them whatever its prototype
schema says. The signature stays wherever that is not certain: a prototype with
no schema document, a scope the schema left open, a key that reached the values
from outside the schema, or a prototype whose application scope another schema
document governs from the environment tree (a `_proto/<proto>/` or
`_apps/<app>/` file) — what that one declares is not in `proto.k`, so nothing
can be closed against it. A value the ytt schema left free-form stays a
`{str:any}` literal default.

`#@schema/validation` bounds become `check:` items in the schema that owns the
field. Reaching into a free-form bag they are guarded by the keys on the way, so
a check never fails on an application that replaced the enclosing scope
wholesale. `not_null=True` becomes `<value> != None`, read from the annotation
itself — ytt's OpenAPI output does not carry it. A custom rule, or a keyword
argument with no KCL counterpart, is not in that block: the migration warns
naming the path of each, which is where to restate it by hand.

A KCL check runs where the schema is instantiated, that is where the application
is declared, and again on every override of that instance, while ytt validated
the final data values of a render once. So a value the prototype validates
without supplying a satisfying default — `min_len=1` on an empty string, which
is how a ytt prototype demands a value — is declared **without a default** and
its check guarded against the absence. Where every declaration of the
prototype's applications states the value on its own level, it is a required
attribute (`image: str`): an application that leaves it out fails to evaluate,
as it failed to render before. KCL checks a required attribute once the
declaring level is complete, so a value that only a lower level sets stays
optional (`image?: str` above): absent until some level sets it, and validated
from that level down. What this does not catch is an application that never sets
it at all. If such a value must be present, assert it in the leaf finalizer.

One consequence to watch during migration: the value no longer defaults to the
empty string, so an application that legacy rendering resolved to `""` gets that
empty string frozen as a leaf patch — and then fails the check. That is a real
invalid configuration, surfaced; fix it by setting the value.

## Known limitations

The converter warns about each of these when it detects them:

- **`environment.*` extras.** The engine owns only `environment.id` and
  `environment.applications`; it regenerates both from the tree. Other keys of
  the scope are ordinary data and are carried over, so templates reading
  `data.values.environment.<key>` keep working.
- **Mapping key order.** KCL emits mapping keys sorted, while ytt kept file
  order. Values are unchanged, but a template that dumps a whole data-values
  subtree (`yaml.encode`, a Helm values file, a ConfigMap body) renders its keys
  in a different order — and anything hashing that text (a `checksum/config`
  annotation) changes with it.
- **Array semantics.** ytt overlays an array element by element and appends what
  is left over, while KCL dict union _replaces_ lists. The generated bridge
  files state KCL's meaning — every array they carry is annotated
  `#@overlay/replace` — so a level that shortens or clears a list gets what it
  wrote. One case is out of reach: a key the engine's data schema does not
  declare is carried in the generated _schema_ document, where ytt drops a
  `replace` that matches nothing, so two levels stating the same undeclared
  array still merge element by element. The converter reports such a value;
  trust the gate.
- **Merge-order skew.** Legacy merges all `_proto` files (all levels) before all
  `_apps` files; the converted tree merges both per level. Values differing
  because of this are corrected by the leaf patches.
- **Arrays in custom scopes.** Custom (non-engine) scopes travel through a
  generated ytt schema document, which cannot carry multi-element arrays. Same
  limitation as legacy app-data schema files.
