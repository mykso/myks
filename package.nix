{
  pkgs,
  self,
}: let
  baseVersion = builtins.readFile "${self}/version.txt";
  commit = self.shortRev or self.dirtyShortRev or "unknown";
  version = "${baseVersion}-${commit}";
in
  pkgs.buildGoModule {
    pname = "myks";
    src = ./.;
    vendorHash = "sha256-vAzDvJXoiE3wdbjQGyyMuuRdIFkn3lL8gdPnlbUUwmQ=";
    version = version;

    env.CGO_ENABLED = 0;
    doCheck = false;
    ldflags = [
      "-s"
      "-w"
      "-X=main.version=${baseVersion}"
      "-X=main.commit=${commit}"
      "-X=main.date=1970-01-01"
    ];

    meta = {
      changelog = "https://github.com/mykso/myks/blob/${baseVersion}/CHANGELOG.md";
      description = "Configuration framework for Kubernetes applications";
      homepage = "https://github.com/mykso/myks";
      license = pkgs.lib.licenses.mit;
    };
  }
