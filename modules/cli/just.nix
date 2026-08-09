/*
  just — command runner with a justfile syntax; simpler make alternative
  https://github.com/casey/just
*/
_: {
  den.aspects.cli._.just = {
    aspect-name = "cli.just";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.just ];
      };
  };
}
