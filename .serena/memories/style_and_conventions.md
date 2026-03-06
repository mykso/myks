# Code style and conventions (myks)

- **Formatting**: gofumpt (stricter than gofmt). Run `task go:fmt` before committing.
- **Linting**: golangci-lint, go vet, gosec (G304 excluded).
- **Logging**: zerolog with structured fields; use trace/debug/info/warn/error appropriately.
- **Errors**: Always check and propagate; wrap with context via `fmt.Errorf`.
- **Concurrency**: golang.org/x/sync; all code must pass `go test -race`.
- **Tests**: Table-driven with testify; `*_test.go` alongside source; fixtures in testData/, examples/integration-tests/.
- **Struct defaults**: Use `creasty/defaults` tags.
- **Commits**: Conventional Commits (enforced by commitlint).
- **User preferences**: Use `rg` instead of grep, `fd` instead of find; yq in PATH is mikefarah/yq.