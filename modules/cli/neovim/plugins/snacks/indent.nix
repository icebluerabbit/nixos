_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.indent = {
    aspect-name = "cli.neovim.plugins.snacks.indent";
    homeManager = _: {
      programs.nixvim.plugins.snacks.settings.indent = {
        enabled = true;
        animate = {
          enabled = true;
          style = "out";
          easing = "linear";
          duration = {
            step = 20;
            total = 500;
          };
        };
        indent = {
          enabled = true;
          char = "┊";
          hl = "SnacksIndent";
        };
        scope = {
          enabled = true;
          priority = 200;
          char = "┋";
          underline = false;
          only_current = false;
          hl = "SnacksIndentScope";
        };
        chunk = {
          enabled = true;
          only_current = true;
          priority = 200;
          hl = "SnacksIndentChunk";
          char = {
            # corner_top = "╭";
            # corner_bottom = "╰";
            # horizontal = "─";
            # vertical = "│";
            # arrow = "─";
            corner_top = "┏";
            corner_bottom = "┗";
            horizontal = "━";
            vertical = "┃";
            arrow = "━";
          };
        };
      };
    };
  };
}
