/*
  Material Symbols — variable icon font by Google superseding Material Icons; adjustable weight/fill/grade axes
  https://github.com/google/material-design-icons
*/
_: {
  den.aspects.fonts._.material-symbols = {
    aspect-name = "fonts.material-symbols";
    nixos =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.material-symbols
        ];
      };
  };
}
