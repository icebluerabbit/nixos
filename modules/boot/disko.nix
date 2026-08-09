/*
  disko — declarative disk partitioning and formatting
  https://github.com/nix-community/disko
*/
{ inputs, ... }:
{
  den.aspects.boot._.disko = {
    aspect-name = "boot.disko";
    nixos =
      { ... }:
      {
        imports = [ inputs.disko.nixosModules.disko ];
      };
  };

  flake-file.inputs.disko = {
    url = "github:nix-community/disko/latest";
    inputs.nixpkgs.follows = "nixpkgs";
  };
}
