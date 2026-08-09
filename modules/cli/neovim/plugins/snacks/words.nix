_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.words = {
    aspect-name = "cli.neovim.plugins.snacks.words";
    homeManager = _: {
      programs.nixvim.plugins.snacks.settings.words = {
        enabled = true;
        debounce = 200;
        notify_jump = false;
        notify_end = true;
        foldopen = true;
        jumplist = true;
        modes = [
          "n"
          "i"
          "c"
        ];
      };

      programs.nixvim.keymaps = [
        {
          key = "]]";
          action = "<cmd>lua Snacks.words.jump(vim.v.count1)<cr>";
          options = {
            desc = "Next Reference";
          };
          mode = [
            "n"
            "t"
          ];
        }
        {
          key = "[[";
          action = "<cmd>lua Snacks.words.jump(-vim.v.count1)<cr>";
          options = {
            desc = "Prev Reference";
          };
          mode = [
            "n"
            "t"
          ];
        }
      ];
    };
  };
}
