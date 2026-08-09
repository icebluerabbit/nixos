# neovim: colorschemes — active colorscheme selection and colorscheme plugin definitions
_: {
  den.aspects.cli._.neovim._.colorschemes = {
    aspect-name = "cli.neovim.colorschemes";
    homeManager = _: {
      programs.nixvim.colorscheme = null;
      programs.nixvim.colorschemes = { };
    };
  };
}
