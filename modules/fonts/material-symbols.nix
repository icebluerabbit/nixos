/*
  Material Symbols — variable icon font by Google superseding Material Icons; adjustable weight/fill/grade axes
  https://github.com/google/material-design-icons
*/
_: {
  den.aspects.fonts._.material-symbols = {
    aspect-name = "fonts.material-symbols";
    # `os` forwards into both nixos and darwin; its route policy fires in
    # user scopes too, so reaching this from a user aspect still lands in
    # the host's system-level fonts.packages.
    os =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.material-symbols
        ];
      };
  };
}
