{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.lz-n = {
    aspect-name = "cli.neovim.plugins.lz-n";
    includes = [
      den.aspects.cli._.neovim._.plugins._.mini._.icons
    ];

    homeManager = _: {
      programs.nixvim.plugins.lz-n = {
        enable = true;
      };
    };
  };
}
