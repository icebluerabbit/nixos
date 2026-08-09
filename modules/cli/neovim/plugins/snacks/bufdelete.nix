_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.bufdelete = {
    aspect-name = "cli.neovim.plugins.snacks.bufdelete";
    homeManager = _: {
      programs.nixvim.keymaps = [
        {
          key = "<leader>bd";
          action = "<cmd>lua Snacks.bufdelete()<cr>";
          options = {
            desc = "Delete Buffer";
          };
        }
      ];
    };
  };
}
