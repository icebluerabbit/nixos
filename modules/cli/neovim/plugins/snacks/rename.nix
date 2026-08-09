_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.rename = {
    aspect-name = "cli.neovim.plugins.snacks.rename";
    homeManager = _: {
      programs.nixvim.keymaps = [
        {
          key = "<leader>cR";
          action = "<cmd>lua Snacks.rename.rename_file()<cr>";
          options = {
            desc = "Rename File";
          };
        }
      ];
    };
  };
}
