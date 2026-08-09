{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp._.servers._.jsut = {
    aspect-name = "cli.neovim.lsp.servers.jsut";
    includes = [ den.aspects.dev._.just-lsp ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.nixvim.lsp.servers.just = {
          enable = true;
          package = pkgs.just-lsp;
          config = {
            cmd = [ (lib.getExe pkgs.just-lsp) ];
            filetypes = [ "just" ];
            root_markers = [ ".git" ];
          };
        };
      };
  };
}
