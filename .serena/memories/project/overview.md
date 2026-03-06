# myks — Project Overview

**Purpose**: myks (Manage my yaml for Kubernetes simply) is a Go CLI for managing Kubernetes application configurations across multiple clusters. It orchestrates Carvel tools (vendir, ytt, kbld) for: downloading sources, templating YAML, resolving images, and generating ArgoCD resources.

**Tech stack**: Go, Cobra (CLI), Viper (config), zerolog (logging), vendir/ytt/kbld (embedded), testify (tests). Build/release: goreleaser, task (Taskfile).

**Codebase structure**:
- `cmd/` — CLI commands (cobra): render, init, cleanup, smart-mode, etc.
- `cmd/embedded/` — Embedded vendir/ytt/kbld runners
- `internal/myks/` — Core: Globe, Environment, Application, rendering, sync, plugins
- `docs/` — User documentation
- `examples/` — Example projects and integration test fixtures
- `testData/` — Test fixtures

**Architecture**: Globe (top-level) → Environment (cluster) → Application (deployable unit) → Prototype (template). Pipeline: Sync (vendir) → Helm render → ytt transform → kbld image resolution → ArgoCD generation.

**Pitfalls**: Use embedded ytt/vendir/kbld only; do not modify `examples/*/rendered/`; respect `.myks/` structure; do not remove `replace` directives in go.mod.