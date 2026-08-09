/*
  neovide — graphical user interface client for Neovim
  https://github.com/neovide/neovide
*/
{ den, ... }:
{
  den.aspects.apps._.neovide = {
    aspect-name = "apps.neovide";
    includes = [
      den.aspects.fonts._.jetbrains-mono
    ];

    homeManager = _: {
      programs.neovide = {
        enable = true;
      };
    };
  };
}
