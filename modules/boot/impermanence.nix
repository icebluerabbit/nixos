/*
  impermanence — declarative persistence for ephemeral root storage
  https://github.com/nix-community/impermanence
*/
{ inputs, ... }:
{
  den.aspects.boot._.impermanence = {
    aspect-name = "boot.impermanence";
    nixos =
      { ... }:
      {
        imports = [ inputs.impermanence.nixosModules.impermanence ];
      };
  };

  flake-file.inputs.impermanence = {
    url = "github:nix-community/impermanence";
    inputs.nixpkgs.follows = "nixpkgs";
    inputs.home-manager.follows = "home-manager";
  };
}
