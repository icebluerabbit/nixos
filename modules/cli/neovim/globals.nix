# neovim: globals — vim.g global variables (leader key, provider flags, plugin globals)
_: {
  den.aspects.cli._.neovim._.globals = {
    aspect-name = "cli.neovim.globals";
    homeManager = _: {
      programs.nixvim.globals = {
        mapleader = " ";
        maplocalleader = " ";
      };
    };
  };
}
