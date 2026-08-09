# neovim: keymaps — key mappings across modes (normal, insert, visual, terminal)
_: {
  den.aspects.cli._.neovim._.keymaps = {
    aspect-name = "cli.neovim.keymaps";
    homeManager = _: {
      programs.nixvim.keymaps = [ ];

      programs.nixvim.keymapsOnEvents = { };
    };
  };
}
