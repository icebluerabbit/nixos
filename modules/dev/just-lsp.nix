/*
  just-lsp — Language Server Protocol (LSP) server for justfiles
  https://github.com/skydiver/just-lsp
*/
_: {
  den.aspects.dev._.just-lsp = {
    aspect-name = "dev.just-lsp";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.just-lsp ];
      };
  };
}
