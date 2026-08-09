{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.conform = {
    aspect-name = "cli.neovim.plugins.conform";
    includes = [
      den.aspects.dev._.nixfmt
      den.aspects.dev._.taplo
      den.aspects.dev._.prettier
      den.aspects.dev._.shfmt
    ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.nixvim.plugins.conform-nvim = {
          enable = true;

          settings = {
            notify_on_error = true;
            notify_no_formatters = true;

            format_on_save = {
              lsp_format = "prefer";
              timeout_ms = 500;
            };

            formatters_by_ft = {
              nix = [ "nixfmt" ];
              toml = [ "taplo" ];
              json = [ "prettier" ];
              yaml = [ "prettier" ];
              markdown = [ "prettier" ];
              sh = [ "shfmt" ];
              bash = [ "shfmt" ];
            };

            formatters = {
              nixfmt.command = lib.getExe pkgs.nixfmt;
              taplo.command = lib.getExe pkgs.taplo;
              prettier.command = lib.getExe pkgs.prettier;
              shfmt.command = lib.getExe pkgs.shfmt;
            };
          };
        };
      };
  };
}
