_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.statuscolumn = {
    aspect-name = "cli.neovim.plugins.snacks.statuscolumn";
    homeManager = _: {
      programs.nixvim.plugins.snacks.settings.statuscolumn = {
        enabled = true;
        left = [
          "mark"
          "sign"
        ];
        right = [
          "fold"
          "git"
        ];
        folds = {
          open = false;
          git_hl = false;
        };
        git = {
          patterns = [
            "GitSign"
            "MiniDiffSign"
          ];
        };
        refresh = 50;
      };
    };
  };
}
