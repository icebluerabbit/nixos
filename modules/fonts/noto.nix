/*
  Noto — pan-Unicode font family by Google; serif, sans-serif, and wide script coverage with harmonized metrics
  https://github.com/notofonts/noto-fonts
*/
_: {
  den.aspects.fonts._.noto = {
    aspect-name = "fonts.noto";
    # `os` forwards into both nixos and darwin; its route policy fires in
    # user scopes too, so reaching this from a user aspect still lands in
    # the host's system-level fonts.packages.
    os =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.noto-fonts
          pkgs.noto-fonts-cjk-sans
        ];
      };
  };
}
