{
  description = "Configuration framework for Kubernetes applications";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        package = import ./nix/package.nix {
          inherit pkgs self;
        };
      in
      {
        packages.default = package;
        packages.myks = package;
        packages.nix-update = pkgs.nix-update;

        devShells.default = import ./nix/shell.nix { inherit pkgs; };
      }
    );
}
