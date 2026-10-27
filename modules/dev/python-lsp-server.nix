/*
  python-lsp-server — community LSP server for Python (pylsp)
  https://github.com/python-lsp/python-lsp-server
*/
_: {
  den.aspects.dev._.python-lsp-server = {
    aspect-name = "dev.python-lsp-server";
    homeManager =
      { pkgs, ... }:
      {
        # Lives under python3Packages, not top level: it is a Python
        # application whose plugins resolve against the same interpreter.
        home.packages = [ pkgs.python3Packages.python-lsp-server ];
      };
  };
}
