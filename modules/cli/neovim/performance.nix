# neovim: performance — runtime tuning; enables Lua bytecode compilation via impatient.nvim/lazy loading
_: {
  den.aspects.cli._.neovim._.performance = {
    aspect-name = "cli.neovim.performance";
    homeManager = _: {
      programs.nixvim.performance = {
        byteCompileLua.enable = true;
      };
    };
  };
}
