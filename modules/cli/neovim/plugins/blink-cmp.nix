{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.blink-cmp = {
    aspect-name = "cli.neovim.plugins.blink-cmp";
    includes = [ den.aspects.cli._.neovim._.lsp ];

    homeManager = _: {
      programs.nixvim.plugins.blink-cmp = {
        enable = true;
        lazyLoad.settings.events = [ "InsertEnter" ];
        settings = {
          keymap = {
            preset = "enter";
          };

          cmdline = {
            keymap.preset = "inherit";
            completion = {
              list.selection.preselect = false;
              menu.auto_show = true;
            };
          };

          completion = {
            accept = {
              auto_brackets = {
                enabled = true;
                semantic_token_resolution = {
                  enabled = true;
                };
              };
            };
            list.selection = {
              preselect = false;
            };
            documentation = {
              auto_show = true;
              auto_show_delay_ms = 50;
            };
            menu.draw.treesitter = [ "lsp" ];
          };

          signature = {
            enabled = true;
            window.show_documentation = true;
          };

          sources = {
            cmdline = [ ];
            providers = {
              buffer = {
                score_offset = -7;
              };
              lsp = {
                fallbacks = [ ];
              };
            };
          };
        };
      };
    };
  };
}
