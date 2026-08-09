_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.zen = {
    aspect-name = "cli.neovim.plugins.snacks.zen";
    homeManager = _: {
      programs.nixvim.plugins.snacks.settings.zen = {
        enabled = true;
        toggles = {
          dim = true;
          git_signs = false;
          mini_diff_signs = false;
        };
        center = true;
        show = {
          statusline = false;
          tabline = false;
        };
        win = {
          style = "zen";
        };
        zoom = {
          toggles = { };
          center = false;
          show = {
            statusline = true;
            tabline = true;
          };
          win = {
            backdrop = false;
            width = 0;
          };
        };
      };

      programs.nixvim.keymaps = [
        {
          key = "<leader>z";
          action = "<cmd>lua Snacks.zen()<cr>";
          options = {
            desc = "Toggle Zen Mode";
          };
        }
        {
          key = "<leader>Z";
          action = "<cmd>lua Snacks.zen.zoom()<cr>";
          options = {
            desc = "Toggle Zoom";
          };
        }
      ];
    };
  };
}
