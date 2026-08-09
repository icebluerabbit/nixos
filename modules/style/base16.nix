/*
  base16-schemes — collection of Base16 color scheme YAML files used as theming input for Stylix
  https://github.com/tinted-theming/schemes
*/
_: {
  den.aspects.style._.base16 = {
    aspect-name = "style.base16";
    nixos =
      { pkgs, ... }:
      {
        environment.systemPackages = [
          pkgs.base16-schemes
        ];
      };
  };
}
