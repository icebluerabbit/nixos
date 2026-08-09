_: {
  den.aspects.cli._.neovim._.plugins._.todo-comments = {
    aspect-name = "cli.neovim.plugins.todo-comments";
    includes = [ ];

    homeManager = _: {
      programs.nixvim.plugins.todo-comments = {
        enable = true;
        lazyLoad = {
          enable = true;
          settings.event = [
            "BufReadPost"
            "BufNewFile"
            "BufWritePre"
          ];
        };
      };
    };
  };
}
