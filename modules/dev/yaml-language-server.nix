/*
  yaml-language-server — Language Server Protocol (LSP) server for YAML
  https://github.com/redhat-developer/yaml-language-server
*/
_: {
  den.aspects.dev._.yaml-language-server = {
    aspect-name = "dev.yaml-language-server";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.yaml-language-server ];
      };
  };
}
