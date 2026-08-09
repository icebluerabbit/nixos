/*
  just — command runner with a justfile syntax; simpler make alternative
  https://github.com/casey/just
*/
_: {
  den.aspects.cli._.tar = {
    aspect-name = "cli.tar";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.gnutar ];
      };
  };
}
