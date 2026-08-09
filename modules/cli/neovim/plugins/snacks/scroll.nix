_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.scroll = {
    aspect-name = "cli.neovim.plugins.snacks.scroll";
    homeManager = _: {
      programs.nixvim.plugins.snacks.settings.scroll = {
        enabled = true;
        animate = {
          duration = {
            step = 10;
            total = 200;
          };
          easing = "linear";
        };
        animate_repeat = {
          delay = 100;
          duration = {
            step = 5;
            total = 50;
          };
          easing = "linear";
        };
      };
    };
  };
}
