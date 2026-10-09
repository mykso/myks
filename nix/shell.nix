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
  ];
  shellHook = ''
    mise install
    source <(mise activate)
  '';
}
