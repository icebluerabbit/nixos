{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.which-key = {
    aspect-name = "cli.neovim.plugins.which-key";
    includes = [
      den.aspects.cli._.neovim._.plugins._.mini._.icons
    ];

    homeManager = _: {
      programs.nixvim.plugins.which-key = {
        enable = true;
        lazyLoad = {
          enable = true;
          settings.event = "DeferredUIEnter";
        };
        settings = {
          win = {
            border = "rounded";
          };
        };
      };
    };
  };
}
