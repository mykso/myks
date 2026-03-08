# AGENTS.md — myks

## Project Overview

**myks** (Manage my yaml for Kubernetes simply) is a Go CLI tool for managing
Kubernetes application configurations across multiple clusters. It orchestrates
Carvel tools (vendir, ytt, kbld) for downloading sources, templating YAML,
resolving images, and generating ArgoCD resources.

## Quick Reference

```bash
# Build
task go:build                    # Build with goreleaser (recommended)
goreleaser build --snapshot --clean --single-target --output bin/myks

# Test (builds binary first, needs it on PATH)
task go:test                     # Build + test with race detector
go test -failfast -race ./...    # Direct (ensure bin/ is on PATH)

# Lint
task go:lint                     # All linters in parallel
golangci-lint run                # Just golangci-lint
go vet ./...                     # Just go vet
gosec -exclude=G304 ./...        # Security scan (G304 excluded: false positive)

# Format
task go:fmt                      # goimports-reviser + gofumpt
goimports-reviser -rm-unused -set-alias -format ./...
gofumpt -w .
```

## Project Structure

```
cmd/                    CLI commands (cobra). Entry points for render, init, cleanup, smart-mode, etc.
cmd/embedded/           Embedded vendir/ytt/kbld tool runners
internal/myks/          Core logic: Globe, Environment, Application, rendering, sync, plugins
docs/                   User documentation
examples/               Example projects and integration test fixtures
testData/               Test fixtures
```

## Architecture

- **Globe**: Top-level container holding all environments and global config
- **Environment**: Represents a Kubernetes cluster with its configuration
- **Application**: Deployable unit in an environment, linked to a Prototype
- **Prototype**: Reusable template (Helm charts, ytt overlays, static YAML)

**Rendering pipeline**: Sync (vendir) → Helm render → ytt transform → kbld image
resolution → ArgoCD generation

## Code Conventions

- **Formatting**: `gofumpt` (stricter than gofmt)
- **Linting**: `golangci-lint`, `go vet`, `gosec`
- **Logging**: `zerolog` with structured fields. Use appropriate levels
  (trace/debug/info/warn/error)
- **Errors**: Always check and propagate. Wrap with context via `fmt.Errorf`
- **Concurrency**: `golang.org/x/sync` utilities. All code must pass
  `go test -race`
- **Testing**: Table-driven tests with `testify`. Tests live alongside source as
  `*_test.go`
- **Struct defaults**: Use `creasty/defaults` tags
- **Commit messages**: Conventional Commits (enforced by commitlint)

## Testing

Tests require the built binary on PATH (task go:test handles this
automatically):

```bash
export PATH="$PWD/bin:$PATH"
go test -failfast -race ./...
go test -race ./internal/myks -run TestSpecificTest
```

Test fixtures are in `testData/`. Integration test fixtures are in
`examples/integration-tests/`.

## Key Dependencies

| Dependency                  | Purpose                  |
| --------------------------- | ------------------------ |
| cobra                       | CLI framework            |
| viper                       | Configuration management |
| zerolog                     | Structured logging       |
| vendir (embedded)           | External source sync     |
| ytt (embedded, custom fork) | YAML templating          |
| kbld (embedded)             | Image resolution         |
| testify                     | Test assertions          |

## CI/CD

- **PR**: lint + test + nix update (GitHub Actions)
- **Release**: release-please + goreleaser (multi-platform binaries, Docker,
  packages)
- Git hooks via `lefthook` (pre-commit: vet/fmt/lint, commit-msg: commitlint)

## Common Pitfalls

- Don't call external ytt/vendir/kbld — use embedded versions
- Don't modify `examples/*/rendered/` — these are generated outputs
- The `.myks/` directory is a service/cache directory — respect its structure
- `go.mod` has `replace` directives for custom Carvel forks — don't remove them

## Code research and editing

Use Serena tools for code research and editing tasks if Serena MCP is available.
Check which code introspection and editing tools are available and use them
appropriately to reduce context usage.
