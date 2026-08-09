# neovim: extra-files — additional runtime files injected into the Neovim config directory
_: {
  den.aspects.cli._.neovim._.extra-files = {
    aspect-name = "cli.neovim.extra-files";
    homeManager = _: {
      programs.nixvim.extraFiles = { };
    };
  };
}
