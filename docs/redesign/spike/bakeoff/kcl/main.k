"""Entrypoint — emits the whole resolved env tree (the ytt-data-values replacement).

  nix shell nixpkgs#kcl -c kcl run .          # from kcl/
"""

import envs.stage as stage

environment = stage.env
