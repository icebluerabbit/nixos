/*
  marksman — Language Server Protocol (LSP) server for Markdown
  https://github.com/artempyanykh/marksman
*/
_: {
  den.aspects.dev._.marksman = {
    aspect-name = "dev.marksman";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.marksman ];
      };
  };
}
