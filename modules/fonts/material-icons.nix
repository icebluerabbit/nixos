/*
  Material Icons — Google's Material Design icon set as a ligature glyph font
  https://github.com/google/material-design-icons
*/
_: {
  den.aspects.fonts._.material-icons = {
    aspect-name = "fonts.material-icons";
    nixos =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.material-icons
        ];
      };
  };
}
