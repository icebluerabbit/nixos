{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.lazydev = {
    aspect-name = "cli.neovim.plugins.lazydev";
    includes = [ den.aspects.cli._.git ];

    homeManager =
      { pkgs, ... }:
      {
        programs.nixvim = {
          plugins = {
            lazydev = {
              enable = true;
              lazyLoad.settings.ft = [ "lua" ];
              settings.libraries = [
                {
                  path = pkgs.fetchFromGitHub {
                    owner = "gonstoll";
                    repo = "wezterm-types";
                    rev = "45ef8d4d98d27be3ec2e472adde4b31df1d6edcb";
                    hash = "sha256-kQJ7hzMAj7lbM83kZAqcslte1EqSY/2R6oSt5s0K/V0=";
                  };
                  mods = [ "wezterm" ];
                }
                {
                  path = "\${3rd}/luv/library";
                  words = [ "vim%.uv" ];
                }
              ];
            };
            blink-cmp.settings.sources.providers.lazydev = {
              name = "LazyDev";
              module = "lazydev.integrations.blink";
              score_offset = 100;
            };
          };
          keymaps = [
            {
              mode = "n";
              key = "<leader>lld";
              action = "<CMD>LazyDev debug<CR>";
              options.desc = "LazyDev debug";
            }
            {
              mode = "n";
              key = "<leader>lll";
              action = "<CMD>LazyDev lsp<CR>";
              options.desc = "LazyDev lsp";
            }
          ];
        };
      };
  };
}
