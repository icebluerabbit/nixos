_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.scratch = {
    aspect-name = "cli.neovim.plugins.snacks.scratch";
    homeManager = _: {
      programs.nixvim.plugins.snacks.settings.scratch = {
        enabled = true;
        name = "Scratch";
        autowrite = true;
        filekey = {
          cwd = true;
          branch = true;
          count = true;
        };
        win = {
          style = "scratch";
        };
      };

      programs.nixvim.keymaps = [
        {
          key = "<leader>.";
          action = "<cmd>lua Snacks.scratch()<cr>";
          options = {
            desc = "Toggle Scratch Buffer";
          };
        }
        {
          key = "<leader>S";
          action = "<cmd>lua Snacks.scratch.select()<cr>";
          options = {
            desc = "Select Scratch Buffer";
          };
        }
      ];
    };
  };
}
