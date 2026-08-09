# neovim: auto-commands — autoCmd event-triggered commands (autocmds)
_: {
  den.aspects.cli._.neovim._.auto-commands = {
    aspect-name = "cli.neovim.auto-commands";
    homeManager = _: {
      programs.nixvim.autoCmd = [ ];
    };
  };
}
