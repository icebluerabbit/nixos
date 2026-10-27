{ den, lib, ... }:
let
  repo = (import ../../_mk-gitea-repo.nix { inherit lib; }) {
    id = "nixos";
    resourceName = "nixos";
    username = "admin";
    name = "nixos";
    description = "NixOS homelab configuration";
    importId = "1";
    migrationCloneAddress = "https://github.com/icebluerabbit/nixos.git";
    migrationService = "github";
    migrationTokenVariable = "github_token";
  };
in
{
  den.aspects.infra._.gitea._.repos._.nixos = {
    aspect-name = "infra.gitea.repos.nixos";
    includes = [
      den.aspects.infra._.providers._.gitea
    ];

    terranix = _: repo.mkTerranix;
  };
}
