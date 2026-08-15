{ buildGoModule }:
buildGoModule {
  pname = "topology";
  version = "0.1.0";
  src = ./.;
  vendorHash = null; # no external dependencies
  meta.mainProgram = "topology";
}
