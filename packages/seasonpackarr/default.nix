/*
  seasonpackarr — autobrr companion that hardlinks already-downloaded episodes
  into an announced season pack instead of re-downloading it
  https://github.com/nuxencs/seasonpackarr

  Pinned to a tagged release, not `develop`: the branch is mid-rework of the
  import path (per-client `import` policy, always-on torrent parsing) and the
  released config shape here — `preImportPath` + `parseTorrentFile` — is what
  modules/hosts/server/containers/seasonpackarr is written against.
*/
{
  lib,
  buildGoModule,
  fetchFromGitHub,
}:
buildGoModule (finalAttrs: {
  pname = "seasonpackarr";
  version = "0.16.0";

  src = fetchFromGitHub {
    owner = "nuxencs";
    repo = "seasonpackarr";
    tag = "v${finalAttrs.version}";
    hash = "sha256-JhtUjuoXT6T3CNofTHdEb0+K8oQ5W4rhsd2pLXOim5k=";
  };

  vendorHash = "sha256-OZXo2m8qDOtwOQanKe0TOqfrE7peumC2NiJWPLcWkQU=";

  # Mirrors upstream's .goreleaser.yaml so `seasonpackarr version` reports the
  # release rather than the "dev" default in internal/buildinfo.
  ldflags = [
    "-s"
    "-w"
    "-X github.com/nuxencs/seasonpackarr/internal/buildinfo.Version=v${finalAttrs.version}"
  ];

  # Both provider tests query the live TVMaze/TVDB APIs, which the build sandbox
  # has no network for. Everything else in the metadata package (the cache) runs.
  checkFlags = [ "-skip=^(Test_TVMaze_EpisodesInSeason|Test_TVDB_EpisodesInSeason)$" ];

  meta = {
    description = "autobrr companion that hardlinks downloaded episodes into announced season packs";
    homepage = "https://github.com/nuxencs/seasonpackarr";
    license = lib.licenses.gpl2Plus;
    mainProgram = "seasonpackarr";
  };
})
