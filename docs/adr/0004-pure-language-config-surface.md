---
status: accepted
---

# The user-editable config surface is pure KCL with language-native inheritance

The configuration layer's **user surface** — every environment level, prototype, and application
config — is authored **in the configuration language itself** (KCL, per ADR 0002), not as plain YAML
files folded by the engine. Inheritance is **language-native**: each level imports/extends its
parent, apps import their prototype, and the whole tree is one import graph evaluated in a single
pass (ADR 0003). This supersedes ADR 0002's original "user-facing layer files stay plain YAML"
framing, which was chosen under the old byte-identical/minimal-migration gate.

## Why

Decided by the bake-off (`docs/redesign/bakeoff-results.md`), where the pure-language
surface was a precondition for measuring the ×2 rubric dimensions at the surface users actually
edit, and it survived:

- **Readability and LSP only exist in-language.** Plain YAML folded by the engine gives no
  completion, no go-to-definition across the inheritance chain, no type errors in-editor — the #1
  user pain (L6) stays unsolved at the surface where users spend their time. The hands-on editor
  pass (2026-08-23) confirmed the KCL language server delivers these over the spike tree.
- **The engine-fold tax disappears.** With in-language inheritance
  (`parent.env { overrides; apps += [...] }`), last-wins scalars, list append, and nested-path
  overrides are native — the hand-written recursive deep-merge required to fold `yaml.decode`'d
  files (ADR 0002's `merge.k`) vanishes.
- **Introspection falls out for free.** L2/L3/L4 (self-reference, cross-stage, cross-application)
  resolve as ordinary references/comprehensions in one eval scope — no shared-key or double-toggle
  workarounds.
- **Import boilerplate is acceptable.** Measured at ~2–3 lines per level/app (the rubric signal
  requirements.md demanded). KCL has no subpackage auto-discovery, so adding an app costs a dir +
  an import line + a roster entry; if this grows too heavy at scale, the engine can layer
  convention-based discovery back on top (the scale spike measured the tax as acceptable — see
  `docs/redesign/bakeoff-results.md`).

## The one deliberate exception: static helm values

Helm values copied verbatim from upstream chart docs stay **plain YAML** (embedded/passed through);
the language owns helm values only when they need logic (cross-reference, conditionals,
introspection). Plain YAML is near-unbeatable UX for the copy-paste case.

## Consequences

- `_apps/` as a directory convention is droppable: with explicit in-language wiring, the import
  statements disambiguate sub-environments from applications — the directory name no longer has to.
- Package/dir names must be language-identifier-safe (`central_forwarder/`, not
  `central-forwarder/`); display names live in the config body.
- Flat vs per-app file layout (tied at 44.0 in the bake-off) was settled by the scale spike's A/B:
  **flat is the default, per-app directories are a per-app opt-in**; both coexist in one tree.
- Migration tooling matters more: hundreds of plain-YAML data-values files per repo are converted
  (machine-seeded via `kcl import`), not merely re-parented.
