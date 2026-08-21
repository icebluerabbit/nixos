/*
  bindery — book/audiobook automation (Readarr replacement): monitors authors
  and series, searches Torznab/Newznab indexers, imports via hardlink.
  https://github.com/vavallee/bindery
*/
{
  lib,
  buildGoModule,
  buildNpmPackage,
  fetchFromGitHub,
}:
let
  version = "1.31.0";

  src = fetchFromGitHub {
    owner = "vavallee";
    repo = "bindery";
    tag = "v${version}";
    hash = "sha256-/ihcANTjhswzlDV2/IScLB69NzGtIQ+MsGSds+yYzSY=";
  };

  # internal/webui/dist ships as a bare .gitkeep — upstream's CI builds the
  # frontend and drops it there before goreleaser runs, so a source build has
  # to reproduce that step rather than rely on the checkout.
  webui = buildNpmPackage {
    pname = "bindery-webui";
    inherit version src;
    sourceRoot = "${src.name}/web";
    npmDepsHash = "sha256-mFoaIS2QOTRDmEnJU7/h/S2vmKbmVoQG4KrnqyrZ3T8=";

    installPhase = ''
      runHook preInstall
      cp -r dist $out
      runHook postInstall
    '';
  };
in
buildGoModule {
  pname = "bindery";
  inherit version src;

  vendorHash = "sha256-qD/eVj4nbgcuVH8Nx2pvjm7T7mdXXA1fko9rVjVVtI8=";

  subPackages = [ "cmd/bindery" ];

  # go.mod asks for `toolchain go1.26.6` against nixpkgs' 1.26.1. The `go`
  # directive is only 1.25.11, so pinning to the local toolchain satisfies the
  # module without letting the build reach out for a newer one.
  env.GOTOOLCHAIN = "local";

  # Matches upstream's goreleaser ldflags; without them the binary reports
  # version "dev" in its logs and UI.
  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];

  preBuild = ''
    cp -r ${webui}/. internal/webui/dist/
  '';

  meta = {
    description = "Book and audiobook automation — monitors authors and series, imports via hardlink";
    homepage = "https://github.com/vavallee/bindery";
    license = lib.licenses.mit;
    mainProgram = "bindery";
  };
}
