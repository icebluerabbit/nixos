/*
  Inter — humanist sans-serif typeface by Rasmus Andersson, optimized for UI text at small sizes on screens
  https://github.com/rsms/inter
*/
_: {
  den.aspects.fonts._.inter = {
    aspect-name = "fonts.inter";
    nixos =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.inter
        ];
      };
  };
}
