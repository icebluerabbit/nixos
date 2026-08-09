_: {
  den.aspects.cli._.neovim._.plugins._.treesitter-context = {
    aspect-name = "cli.neovim.plugins.treesitter-context";
    includes = [ ];

    homeManager = _: {
      programs.nixvim.plugins.treesitter-context = {
        enable = true;
        lazyLoad = {
          enable = true;
          settings.event = [
            "BufReadPre"
            "BufReadPost"
            "BufNewFile"
            "BufEnter"
          ];
        };
        settings.max_lines = 7;
      };
    };
  };
}
