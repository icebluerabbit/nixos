# neovim: auto-groups — named augroups for organizing autocmds
_: {
  den.aspects.cli._.neovim._.auto-groups = {
    aspect-name = "cli.neovim.auto-groups";
    homeManager = _: {
      programs.nixvim.autoGroups = {
        filetypes = { };
      };
    };
  };
}
