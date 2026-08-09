# neovim: extra-plugins — raw plugin packages not managed by nixvim's plugin system
_: {
  den.aspects.cli._.neovim._.extra-plugins = {
    aspect-name = "cli.neovim.extra-plugins";
    homeManager = _: {
      programs.nixvim.extraPlugins = [ ];
    };
  };
}
