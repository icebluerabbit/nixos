{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp._.servers._.nix = {
    aspect-name = "cli.neovim.lsp.servers.nix";
    includes = [ den.aspects.dev._.nixd ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.nixvim.lsp.servers.nixd = {
          enable = true;
          package = pkgs.nixd;
          config = {
            cmd = [ (lib.getExe pkgs.nixd) ];
            filetypes = [ "nix" ];
            root_markers = [ ".git" ];
          };
        };
      };
  };
}
