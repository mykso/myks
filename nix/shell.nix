{ pkgs }:
pkgs.mkShell {
  packages = with pkgs; [
    gnused
    go
    go-task
    gofumpt
    goimports-reviser
    goreleaser
    gosec
    lefthook
    mise
    nix-update
    upx
  ];
  shellHook = ''
    mise install
    source <(mise activate)
  '';
}
