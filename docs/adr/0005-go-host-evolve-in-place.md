---
status: accepted
---

# Keep Go as the host language and evolve myks in place

Closes the sub-decision left open by ADR 0001. The redesigned renderer is built by **evolving the
existing Go codebase in place** — no rewrite, no host-language change. The result ships as a new
**major version** with a clean configuration break (ADR 0004); the previous major is frozen on a
branch and receives bugfixes and trivial features on demand for **12 months** from the first
stable new-major release.

## Why Go

- **KCL's best-supported embedding surface is the Go SDK.** `kcl-lang.io/kcl-go` ships the Rust
  core as a prebuilt native library loaded via purego (no CGO), covers every platform we release
  for, and is exercised in production (Kusion, Crossplane's function-kcl, KCL's own CLI).
- **The embedded toolchain is Go.** ytt (retained for match-and-mutate overlays, ADR 0002), vendir
  and kbld are linked in-process as Go libraries. Any other host loses the single-binary property
  or re-implements them.
- **Contributor pool.** The k8s renderer niche is uniformly Go; myks's existing code, tests, CI
  and contributors are Go.

## Why not Rust (considered, rejected)

A Rust rewrite was evaluated seriously — KCL's core is Rust, and owning improved vendir/kbld
functionality in-repo was on the table anyway. The facts killed it:

- KCL publishes **no crates on crates.io**; the Rust API is a git-dependency with no stability
  statement. Even KCL's own CLI is Go wrapping the Rust core — native-Rust embedding is the
  least-exercised path.
- **No ytt port exists** in Rust (starlark-rust provides the language, none of ytt's overlay
  semantics); retaining overlays would mean shelling out to a ytt binary or building an overlay
  engine from scratch.
- The Rust YAML ecosystem is fragmented (serde_yaml deprecated 2024; no comment-preserving node
  API comparable to Go's `yaml.v3`).
- Helm rendering is exec-only in both worlds — no advantage either way.
- Zero precedent: no notable k8s-config renderer/orchestrator is written in Rust.

The one argument that favored a rewrite — reimplementing vendir/kbld functionality in-repo — is
language-neutral and is deferred regardless (see roadmap).

## Why evolve in place (not rewrite-in-Go)

ADR 0001 already established myks is ~80% a plugin engine with clean seams. A rewrite re-earns
that 80% for zero user value; in-place evolution ships the config layer as incremental,
reviewable PRs behind per-repo mode detection (ADR 0006), and the legacy path keeps working until
the major-version cut.

## Consequences

- `kcl-lang.io/kcl-go` becomes a dependency: ~16 MB binary growth, version lockstep with the KCL
  language (upgrading the SDK upgrades the language atomically), and a first-run native-lib
  extraction that needs a writable `KCL_LIB_HOME` on read-only rootfs.
- Old and new configuration paths coexist in one binary during the transition (ADR 0006).
- The legacy-branch maintenance promise is bounded and documented at release time.
- vendir/kbld reimplementation stays out of the redesign's critical path.
