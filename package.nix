{
  pkgs,
  self,
}:
let
  baseVersion = builtins.readFile "${self}/version.txt";
  commit = self.shortRev or self.dirtyShortRev or "unknown";
  version = "${baseVersion}-${commit}";
in
pkgs.buildGoModule {
  pname = "myks";
  src = ./.;
  vendorHash = "sha256-G3qqISWk274E0mUX9EN55mnu713Z5IC8Xz1PGnZgxwI=";
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
