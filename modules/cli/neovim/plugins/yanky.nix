{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.yanky = {
    aspect-name = "cli.neovim.plugins.yanky";
    includes = [
      den.aspects.cli._.neovim._.plugins._.snacks._.picker
    ];

    homeManager = _: {
      programs.nixvim.plugins.yanky = {
        enable = true;
        lazyLoad = {
          enable = true;
          settings.event = "DeferredUIEnter";
        };
        settings = {
          ring.history_length = 50;
          highlight.timer = 200;
        };
      };

      programs.nixvim.keymaps = [
        {
          mode = "n";
          key = "<leader>nc";
          action = "<cmd>lua Snacks.picker.yanky()<cr>";
          options.desc = "Yank history";
        }
        {
          mode = [
            "n"
            "x"
          ];
          key = "y";
          action = "<Plug>(YankyYank)";
          options.desc = "Yank";
        }
        {
          mode = [
            "n"
            "x"
          ];
          key = "p";
          action = "<Plug>(YankyPutAfter)";
          options.desc = "Put after";
        }
        {
          mode = [
            "n"
            "x"
          ];
          key = "P";
          action = "<Plug>(YankyPutBefore)";
          options.desc = "Put before";
        }
        {
          mode = [
            "n"
            "x"
          ];
          key = "gp";
          action = "<Plug>(YankyGPutAfter)";
          options.desc = "Put after selection";
        }
        {
          mode = [
            "n"
            "x"
          ];
          key = "gP";
          action = "<Plug>(YankyGPutBefore)";
          options.desc = "Put before selection";
        }
        {
          mode = "n";
          key = "<C-p>";
          action = "<Plug>(YankyPreviousEntry)";
          options.desc = "Cycle yank history back";
        }
        {
          mode = "n";
          key = "<C-n>";
          action = "<Plug>(YankyNextEntry)";
          options.desc = "Cycle yank history forward";
        }
        {
          mode = "n";
          key = "]p";
          action = "<Plug>(YankyPutIndentAfterLinewise)";
          options.desc = "Put indented after (linewise)";
        }
        {
          mode = "n";
          key = "[p";
          action = "<Plug>(YankyPutIndentBeforeLinewise)";
          options.desc = "Put indented before (linewise)";
        }
        {
          mode = "n";
          key = ">p";
          action = "<Plug>(YankyPutIndentAfterShiftRight)";
          options.desc = "Put and indent right";
        }
        {
          mode = "n";
          key = "<p";
          action = "<Plug>(YankyPutIndentAfterShiftLeft)";
          options.desc = "Put and indent left";
        }
        {
          mode = "n";
          key = ">P";
          action = "<Plug>(YankyPutIndentBeforeShiftRight)";
          options.desc = "Put before and indent right";
        }
        {
          mode = "n";
          key = "<P";
          action = "<Plug>(YankyPutIndentBeforeShiftLeft)";
          options.desc = "Put before and indent left";
        }
        {
          mode = "n";
          key = "=p";
          action = "<Plug>(YankyPutAfterFilter)";
          options.desc = "Put after filter";
        }
        {
          mode = "n";
          key = "=P";
          action = "<Plug>(YankyPutBeforeFilter)";
          options.desc = "Put before filter";
        }
      ];
    };
  };
}
