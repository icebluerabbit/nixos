/*
  Noto Color Emoji — full-color emoji font by Google covering the Unicode emoji set
  https://github.com/googlefonts/noto-emoji
*/
_: {
  den.aspects.fonts._.noto-color-emoji = {
    aspect-name = "fonts.noto-color-emoji";
    # `os` forwards into both nixos and darwin; its route policy fires in
    # user scopes too, so reaching this from a user aspect still lands in
    # the host's system-level fonts.packages.
    os =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.noto-fonts-color-emoji
        ];
      };
  };
}
