{ buildGoModule }:
buildGoModule {
  pname = "mousetrap";
  version = "0.1.0";
  src = ./.;
  vendorHash = "sha256-Epg7qf5/RZFz+OAbWftdCni+C9m+uLroD0TdVZXM+k8=";
  meta.mainProgram = "mousetrap";
}
