/*
  shfmt — formatter for shell scripts
  https://github.com/mvdan/sh
*/
_: {
  den.aspects.dev._.shfmt = {
    aspect-name = "dev.shfmt";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.shfmt ];
      };
  };
}
