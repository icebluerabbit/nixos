_: {
  den.aspects.cli._.neovim._.plugins._.treesitter-textobjects = {
    aspect-name = "cli.neovim.plugins.treesitter-textobjects";
    includes = [ ];

    homeManager = _: {
      programs.nixvim.plugins.treesitter-textobjects = {
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
        settings = {
          lsp_interop = {
            enable = true;
            border = "rounded";
          };
          move = {
            enable = true;
            gotoNextStart = {
              "]f" = "@function.outer";
              "]c" = "@class.outer";
              "]a" = "@parameter.inner";
            };
            gotoNextEnd = {
              "]F" = "@function.outer";
              "]C" = "@class.outer";
              "]A" = "@parameter.inner";
            };
            gotoPreviousStart = {
              "[f" = "@function.outer";
              "[c" = "@class.outer";
              "[a" = "@parameter.inner";
            };
            gotoPreviousEnd = {
              "[F" = "@function.outer";
              "[C" = "@class.outer";
              "[A" = "@parameter.inner";
            };
          };
          select = {
            enable = true;
            lookahead = true;

            keymaps = {
              "aa" = "@parameter.outer";
              "ia" = "@parameter.inner";
              "af" = "@function.outer";
              "if" = "@function.inner";
              "ac" = "@class.outer";
              "ic" = "@class.inner";
            };
          };
          swap.enable = true;
        };
      };
    };
  };
}
