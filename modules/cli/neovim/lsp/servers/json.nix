{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp._.servers._.json = {
    aspect-name = "cli.neovim.lsp.servers.json";
    includes = [ den.aspects.dev._.vscode-langservers ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.nixvim.lsp.servers.jsonls = {
          enable = true;
          package = pkgs.vscode-langservers-extracted;
          config = {
            cmd = [
              (lib.getExe' pkgs.vscode-langservers-extracted "vscode-json-language-server")
              "--stdio"
            ];
            filetypes = [ "json" ];
            root_markers = [ ".git" ];
            init_options = {
              provideFormatter = true;
            };
          };
        };
      };
  };
}
