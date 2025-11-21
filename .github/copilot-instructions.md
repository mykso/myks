# GitHub Copilot Instructions for myks

## Project Overview

**myks** (Manage **my** **y**aml for **K**ubernetes **s**imply) is a Go-based tool and framework for managing application configurations across multiple Kubernetes clusters. It enables reusing, mutating, and sharing configurations between applications and environments.

The tool orchestrates:
- External source management via [vendir](https://carvel.dev/vendir/) (Helm charts, Git repos, GitHub releases)
- Helm chart rendering with custom values
- YAML templating and validation via [ytt](https://carvel.dev/ytt/)
- Image resolution via [kbld](https://carvel.dev/kbld/)
- ArgoCD resource generation for GitOps workflows

## Technology Stack

- **Language**: Go 1.24+
- **Key Dependencies**: 
  - Embedded vendir, ytt, and kbld from Carvel
  - Cobra for CLI framework
  - Viper for configuration management
  - zerolog for structured logging
  - stretchr/testify for testing
- **External Tools**: Helm (required for Helm chart rendering)

## Repository Structure

```
.
├── cmd/                  # CLI commands and subcommands
├── internal/myks/        # Core application logic
│   ├── application.go    # Application rendering logic
│   ├── environment.go    # Environment management
│   ├── render*.go        # Rendering pipelines (Helm, ytt, kbld)
│   ├── sync*.go          # Synchronization logic (vendir, Helm)
│   ├── plugin*.go        # Plugin system
│   └── smart_mode.go     # Smart detection of changes
├── docs/                 # User documentation
├── examples/             # Example configurations
├── testData/             # Test fixtures
└── main.go              # Entry point
```

## Code Style and Conventions

### General Go Practices
- Follow standard Go conventions and idioms
- Use `gofumpt` for formatting (stricter than `gofmt`)
- Run `golangci-lint` for linting
- Use `go vet` for static analysis
- Run `gosec` for security checks (excluding G304 - false positive for file inclusion)

### Specific Patterns
- **Error Handling**: Always check and properly propagate errors
- **Logging**: Use `github.com/rs/zerolog` for structured logging with appropriate levels (trace, debug, info, warn, error)
- **Context**: Pass `context.Context` for cancellation support in long-running operations
- **Concurrency**: Use `golang.org/x/sync` utilities for managing concurrent operations (see `async` configuration)

### Naming Conventions
- Use descriptive names that reflect domain concepts (e.g., `Environment`, `Application`, `Globe`)
- Test files follow `*_test.go` pattern
- Table-driven tests with struct-based test cases

### Comments
- Document exported functions, types, and packages
- Avoid redundant comments that merely restate the code
- Use comments to explain "why" rather than "what" when the code isn't self-explanatory

## Development Workflow

### Initial Setup
```bash
# Clone and install dependencies
git clone https://github.com/mykso/myks.git
cd myks
go mod download

# Install optional development tools
# task, lefthook, gofumpt, golangci-lint, commitlint
```

### Building
```bash
# Using task (recommended)
task go:build

# Or directly with Go
go build -o bin/myks ./main.go

# Or with goreleaser
goreleaser build --snapshot --clean --single-target --output bin/myks
```

### Testing
```bash
# Run all tests with race detector
export PATH="$PWD/bin:$PATH"
go test -race ./...

# Run specific test
go test -race ./internal/myks -run TestApplication_renderDataYaml

# Using task (includes build)
task go:test
```

### Linting
```bash
# Run all linters
task go:lint

# Or individually
golangci-lint run
go vet ./...
gosec -exclude=G304 ./...
```

### Formatting
```bash
# Format code
task go:fmt

# Or manually
goimports-reviser -rm-unused -set-alias -format ./...
gofumpt -w .
```

### Git Hooks
The project uses `lefthook` for Git hooks:
- **pre-commit**: Runs `go vet`, `gofumpt`, and `golangci-lint`
- **commit-msg**: Validates commit messages with `commitlint`

Install hooks with: `lefthook install`

## Testing Practices

### Test Structure
- Use table-driven tests with descriptive test names
- Place test files alongside source files (`*_test.go`)
- Use `testData/` directory for test fixtures
- Test files should have a `package myks` declaration

### Example Test Pattern
```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name    string
        args    args
        want    string
        wantErr bool
    }{
        {"happy path", args{...}, "expected", false},
        {"error case", args{...}, "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests
- Located in `internal/integration/`
- May require building the binary first
- Use real file system operations with temporary directories

## Architecture and Key Concepts

### Core Components

1. **Globe**: Represents the entire myks environment (all environments and applications)
2. **Environment**: A Kubernetes cluster/environment with its specific configuration
3. **Application**: A deployable unit within an environment, linked to a prototype
4. **Prototype**: Reusable application template combining Helm charts, ytt templates, and overlays

### Rendering Pipeline
1. **Sync Stage**: Download external sources (vendir), resolve Helm dependencies
2. **Render Stage**: 
   - Render Helm charts with values
   - Apply ytt transformations
   - Resolve images with kbld (optional)
   - Generate ArgoCD resources (optional)

### Configuration System
- Uses `.myks.yaml` for global configuration
- Hierarchical data values: environment → prototype → application
- Supports `config-in-root: true` for running myks from subdirectories

### Smart Mode
- Automatically detects what needs to be rendered based on git changes
- Reduces unnecessary processing for large repositories
- See `docs/smart-mode.md` for details

### Plugin System
- Executables prefixed with `myks-` on PATH or in `plugins/` directory
- Receive environment variables (`MYKS_ENV`, `MYKS_APP`, `MYKS_ENV_DIR`, etc.)
- Can act on rendered manifests (validation, deployment, secret management)

## Important Implementation Details

### Embedded Tools
- ytt, vendir, and kbld are embedded via `cmd/embedded/`
- Custom forks are used (see `replace` directives in `go.mod`)
- The binary checks and runs as embedded tools when appropriate

### Vendir Secrets
- Dynamically creates secrets from `VENDIR_SECRET_*` environment variables
- Format: `VENDIR_SECRET_<NAME>_USERNAME` and `VENDIR_SECRET_<NAME>_PASSWORD`
- Secrets are cleaned up after sync completion

### Parallel Processing
- Configured via `async` setting (0 = unlimited)
- Uses worker pools for concurrent rendering
- Must handle concurrent access to shared resources safely

### File Operations
- Most operations work with temporary directories and `.myks/` cache
- Use `os.WriteFile` and `os.ReadFile` for file I/O
- Respect `rendered/` output directory structure

## Configuration Files

### `.myks.yaml` Structure
```yaml
# Number of applications to process in parallel (0 = unlimited)
async: 4

# Auto-detect root directory from config file location
config-in-root: true

# Logging level: trace, debug, info, warn, error, fatal, panic
log-level: info

# Minimum required myks version
min-version: 'v4.0.0'

# Additional plugin directories
plugin-sources:
  - ./plugins
```

### Application Data Files
- `app-data.ytt.yaml`: Application-specific configuration
- `env-data.ytt.yaml`: Environment-specific configuration
- Data files are processed with ytt schema validation

## Common Pitfalls to Avoid

1. **Don't modify files in `examples/` rendered directories** - these are generated outputs
2. **Always use embedded tool references** - don't call external ytt/vendir/kbld directly when embedded versions exist
3. **Handle nil pointers in config structs** - use `creasty/defaults` for default values
4. **Test with race detector** - concurrent operations must be thread-safe
5. **Respect `.myks/` cache structure** - don't break caching mechanisms
6. **Validate user input** - especially paths and configuration values

## Documentation

- **User Docs**: `docs/` directory
- **Examples**: `examples/` directory with working configurations
- **Inline**: GoDoc comments for exported symbols
- When adding features, update relevant documentation files

## CI/CD

The project uses GitHub Actions:
- Linting and testing on pull requests
- Release automation with goreleaser
- Link checking for documentation

## External Resources

- [Main Documentation](docs/README.md)
- [Configuration Reference](docs/configuration.md)
- [Plugins Guide](docs/plugins.md)
- [Smart Mode](docs/smart-mode.md)
- [Optimization Tips](docs/optimizations.md)
- [kbld Usage](docs/kbld.md)

## Getting Help

- [Slack Channel](https://kubernetes.slack.com/archives/C06BVDBHZC2)
- [GitHub Issues](https://github.com/mykso/myks/issues)
- [GitHub Discussions](https://github.com/mykso/myks/discussions)
