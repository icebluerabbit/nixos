/*
  nixfmt — opinionated formatter for Nix files
  https://github.com/nix-community/nixfmt
*/
_: {
  den.aspects.dev._.nixfmt = {
    aspect-name = "dev.nixfmt";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.nixfmt ];
      };
  };
}
