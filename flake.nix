{
  description = "Configuration framework for Kubernetes applications";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {inherit system;};
      package = import ./package.nix {
        inherit pkgs self;
      };
    in {
      packages.default = package;
      packages.myks = package;

      devShells.default = import ./shell.nix {inherit pkgs package;};
    });
}
