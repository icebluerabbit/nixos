{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.lazydev = {
    aspect-name = "cli.neovim.plugins.lazydev";
    includes = [ den.aspects.cli._.git ];

    homeManager = _: {
      programs.nixvim = {
        plugins = {
          lazydev = {
            enable = true;
            lazyLoad.settings.ft = [ "lua" ];
            settings.libraries = [
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
