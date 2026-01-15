{ pkgs }:
pkgs.mkShell {
  packages = with pkgs; [
    gnused
    go
    go-task
    gofumpt
    goimports-reviser
    golangci-lint
    goreleaser
    gosec
    lefthook
    nix-update
    upx
  ];
}
