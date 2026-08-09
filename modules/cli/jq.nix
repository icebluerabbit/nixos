/*
  jq — lightweight command-line JSON processor with a functional query language
  https://github.com/jqlang/jq
*/
_: {
  den.aspects.cli._.jq = {
    aspect-name = "cli.jq";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.jq ];
      };
  };
}
