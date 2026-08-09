{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp._.servers._.yaml = {
    aspect-name = "cli.neovim.lsp.servers.yaml";
    includes = [ den.aspects.dev._.yaml-language-server ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.nixvim.lsp.servers.yamlls = {
          enable = true;
          package = pkgs.yaml-language-server;
          config = {
            cmd = [
              (lib.getExe pkgs.yaml-language-server)
              "--stdio"
            ];
            filetypes = [ "yaml" ];
            root_markers = [ ".git" ];
            settings = {
              yaml.format.enable = true;
              redhat = {
                telemetry = {
                  enabled = false;
                };
              };
            };
          };
        };
      };
  };
}
