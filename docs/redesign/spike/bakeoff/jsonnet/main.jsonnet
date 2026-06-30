// Entrypoint — emits the whole resolved env tree (the ytt-data-values replacement).
//   nix shell nixpkgs#go-jsonnet -c jsonnet main.jsonnet      # from jsonnet/
{ environment: import 'envs/stage.libsonnet' }
