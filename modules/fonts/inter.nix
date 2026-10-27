/*
  Inter — humanist sans-serif typeface by Rasmus Andersson, optimized for UI text at small sizes on screens
  https://github.com/rsms/inter
*/
_: {
  den.aspects.fonts._.inter = {
    aspect-name = "fonts.inter";
    # `os` forwards into both nixos and darwin; its route policy fires in
    # user scopes too, so reaching this from a user aspect still lands in
    # the host's system-level fonts.packages.
    os =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.inter
        ];
      };
  };
}
