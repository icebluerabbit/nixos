{ buildGoModule }:
buildGoModule {
  pname = "mousetrap";
  version = "0.1.0";
  src = ./.;
  vendorHash = "sha256-8wllMNDAJDCsndE9cP47kPk3LsKb8m550dv1wVh7dvg=";
  meta.mainProgram = "mousetrap";
}
