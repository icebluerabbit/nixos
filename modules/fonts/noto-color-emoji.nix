/*
  Noto Color Emoji — full-color emoji font by Google covering the Unicode emoji set
  https://github.com/googlefonts/noto-emoji
*/
_: {
  den.aspects.fonts._.noto-color-emoji = {
    aspect-name = "fonts.noto-color-emoji";
    nixos =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.noto-fonts-color-emoji
        ];
      };
  };
}
