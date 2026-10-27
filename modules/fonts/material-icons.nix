/*
  Material Icons — Google's Material Design icon set as a ligature glyph font
  https://github.com/google/material-design-icons
*/
_: {
  den.aspects.fonts._.material-icons = {
    aspect-name = "fonts.material-icons";
    # `os` forwards into both nixos and darwin; its route policy fires in
    # user scopes too, so reaching this from a user aspect still lands in
    # the host's system-level fonts.packages.
    os =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.material-icons
        ];
      };
  };
}
