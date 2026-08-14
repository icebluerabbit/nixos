{ den, lib, ... }:
(import ../../_mk-gitea-repo.nix { inherit den lib; }) {
  id = "nixos";
  resourceName = "nixos";
  username = "admin";
  name = "nixos";
  description = "NixOS homelab configuration";
  importId = "1";
  migrationCloneAddress = "https://github.com/icebluerabbit/nixos.git";
  migrationService = "github";
  migrationTokenVariable = "github_token";
}
