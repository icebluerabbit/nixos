{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.conform = {
    aspect-name = "cli.neovim.plugins.conform";
    includes = [
      den.aspects.dev._.nixfmt
      den.aspects.dev._.taplo
      den.aspects.dev._.prettier
      den.aspects.dev._.shfmt
      den.aspects.dev._.stylua
      den.aspects.dev._.go
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
              go = [ "gofmt" ];
              # Not covered by treefmt (modules/formatter.nix), so unlike the
              # entries above this one has no pre-commit hook to agree with.
              lua = [ "stylua" ];
            };

            formatters = {
              nixfmt.command = lib.getExe pkgs.nixfmt;
              taplo.command = lib.getExe pkgs.taplo;
              prettier.command = lib.getExe pkgs.prettier;
              shfmt.command = lib.getExe pkgs.shfmt;
              stylua.command = lib.getExe pkgs.stylua;
              # gofmt, not gofumpt: treefmt runs gofmt (modules/formatter.nix)
              # and the pre-commit hook runs the same wrapper, so a stricter
              # formatter here would fight the hook on every commit. It ships
              # inside the go package rather than as its own derivation, so
              # lib.getExe — which resolves to `go` — is not what is wanted.
              gofmt.command = "${pkgs.go}/bin/gofmt";
            };
          };
        };
      };
  };
}
