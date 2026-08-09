_: {
  den.aspects.cli._.neovim._.plugins._.snacks = {
    aspect-name = "cli.neovim.plugins.snacks";
    homeManager = _: {
      programs.nixvim.plugins.snacks = {
        enable = true;
        # lazyLoad = {
        #   enable = true;
        #   settings.event = [
        #     "DeferredUIEnter"
        #     "BufReadPost"
        #   ];
        # };
      };
    };
  };
}
