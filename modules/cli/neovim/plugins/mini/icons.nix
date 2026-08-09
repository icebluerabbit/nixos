_: {
  den.aspects.cli._.neovim._.plugins._.mini._.icons = {
    aspect-name = "cli.neovim.plugins.mini.icons";
    homeManager = _: {
      programs.nixvim.plugins.mini-icons = {
        enable = true;
        mockDevIcons = true;
        settings = {
          style = "glyph";
        };
      };
    };
  };
}
