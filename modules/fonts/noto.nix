/*
  Noto — pan-Unicode font family by Google; serif, sans-serif, and wide script coverage with harmonized metrics
  https://github.com/notofonts/noto-fonts
*/
_: {
  den.aspects.fonts._.noto = {
    aspect-name = "fonts.noto";
    nixos =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.noto-fonts
          pkgs.noto-fonts-cjk-sans
        ];
      };
  };
}
