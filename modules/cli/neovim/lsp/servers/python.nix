{ den, ... }:
{
  den.aspects.cli._.neovim._.lsp._.servers._.python = {
    aspect-name = "cli.neovim.lsp.servers.python";
    includes = [ den.aspects.dev._.python-lsp-server ];

    homeManager =
      { pkgs, lib, ... }:
      {
        programs.nixvim.lsp.servers.pylsp = {
          enable = true;
          package = pkgs.python3Packages.python-lsp-server;
          config = {
            cmd = [ (lib.getExe' pkgs.python3Packages.python-lsp-server "pylsp") ];
            filetypes = [ "python" ];
            root_markers = [
              "pyproject.toml"
              "setup.py"
              "setup.cfg"
              "requirements.txt"
              "ruff.toml"
              ".git"
            ];
            settings.pylsp.plugins = {
              # ruff is this repo's Python formatter and linter
              # (modules/formatter.nix), and the pre-commit hook runs the
              # same wrapper. pylsp ships its own pycodestyle/autopep8/yapf
              # stack, which would disagree with ruff on every save, so only
              # the parts ruff does not cover stay on.
              pycodestyle.enabled = false;
              autopep8.enabled = false;
              yapf.enabled = false;
              mccabe.enabled = false;
              pyflakes.enabled = false;
            };
          };
        };
      };
  };
}
