{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp._.servers._.markdown = {
    aspect-name = "cli.neovim.lsp.servers.markdown";
    includes = [ den.aspects.dev._.marksman ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.nixvim.lsp.servers.marksman = {
          enable = true;
          package = pkgs.marksman;
          config = {
            cmd = [
              (lib.getExe pkgs.marksman)
              "server"
            ];
            filetypes = [ "markdown" ];
            root_markers = [
              ".git"
              ".marksman.toml"
            ];
          };
        };
      };
  };
}
