_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.lazygit = {
    aspect-name = "cli.neovim.plugins.snacks.lazygit";
    homeManager = _: {
      programs.nixvim.plugins.snacks.settings.lazygit = {
        enabled = true;
        configure = true;
        config = {
          os = {
            editPreset = "nvim-remote";
          };
          gui = {
            nerdFontsVersion = "3";
          };
        };
        win = {
          style = "lazygit";
        };
      };

      programs.nixvim.keymaps = [
        {
          key = "<leader>gg";
          action = "<cmd>lua Snacks.lazygit()<cr>";
          options = {
            desc = "Lazygit";
          };
        }
      ];
    };
  };
}
