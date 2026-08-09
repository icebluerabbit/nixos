# neovim: file-types — filetype detection and per-filetype option overrides
_: {
  den.aspects.cli._.neovim._.file-types = {
    aspect-name = "cli.neovim.file-types";
    homeManager = _: {
      programs.nixvim.filetype = { };
    };
  };
}
