/*
  manix — fast CLI documentation searcher for nixpkgs, NixOS options, and home-manager options
  https://github.com/nix-community/manix
*/
{ inputs, ... }:
{
  den.aspects.dev._.manix = {
    aspect-name = "dev.manix";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [
          pkgs.manix
          pkgs.man-pages
          pkgs.man-pages-posix
        ];
      };

    nixos = _: {
      nix.nixPath = [
        "home-manager=${inputs.home-manager}"
        "nixpkgs=${inputs.nixpkgs}"
      ];
    };
  };
}
