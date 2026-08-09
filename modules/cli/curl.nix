/*
  just — command runner with a justfile syntax; simpler make alternative
  https://github.com/casey/just
*/
_: {
  den.aspects.cli._.curl = {
    aspect-name = "cli.curl";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.curl ];
      };
  };
}
