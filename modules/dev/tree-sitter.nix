/*
  tree-sitter — parser generator tool and incremental parsing library
  https://github.com/tree-sitter/tree-sitter
*/
_: {
  den.aspects.dev._.tree-sitter = {
    aspect-name = "dev.tree-sitter";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.tree-sitter ];
      };
  };
}
