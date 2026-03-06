# Suggested commands (myks)

## Build
- `task go:build` — Build with goreleaser (recommended)
- `goreleaser build --snapshot --clean --single-target --output bin/myks`

## Test (binary must be on PATH)
- `task go:test` — Build + test with race detector
- `export PATH="$PWD/bin:$PATH"` then `go test -failfast -race ./...`
- `go test -race ./internal/myks -run TestName` — Single package/test

## Lint
- `task go:lint` — All linters (golangci-lint, go vet, gosec)
- `golangci-lint run`
- `go vet ./...`
- `gosec -exclude=G304 ./...`

## Format
- `task go:fmt` — goimports-reviser + gofumpt
- `goimports-reviser -rm-unused -set-alias -format ./...`
- `gofumpt -w .`

## Run CLI
- `./bin/myks` or `myks` (when bin is on PATH)

## Utils (Darwin)
- `git`, `ls`, `cd`, `rg` (ripgrep), `fd` (instead of find), `yq` (mikefarah/yq)
- Prefer `rg` over grep, `fd` over find.