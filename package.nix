{
  pkgs,
  self,
}: let
  baseVersion = "4.2.3"; # x-release-please-version
  commit = self.shortRev or self.dirtyShortRev or "unknown";
  version = "${baseVersion}-${commit}";
in
  pkgs.buildGoModule {
    pname = "myks";
    src = ./.;
    vendorHash = "sha256-4iiNd7n+DmTB04Qp9nLY2puG5ItTjnfkk69DOlDydR4=";
    version = version;

    CGO_ENABLED = 0;
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
