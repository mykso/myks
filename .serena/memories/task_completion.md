# When a task is completed (myks)

1. **Format**: Run `task go:fmt` (goimports-reviser + gofumpt).
2. **Lint**: Run `task go:lint` (golangci-lint, go vet, gosec).
3. **Test**: Run `task go:test` (builds binary, then `go test -failfast -race ./...`).

Ensure the binary is on PATH for tests (task go:test does this automatically). Fix any lint or test failures before considering the task done. Pre-commit hooks (lefthook) run vet/fmt/lint and commitlint.