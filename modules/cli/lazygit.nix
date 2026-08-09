/*
  lazygit — terminal UI for git with keyboard-driven staging, branching, and rebasing
  https://github.com/jesseduffield/lazygit
*/
_: {
  den.aspects.cli._.lazygit = {
    aspect-name = "cli.lazygit";
    homeManager =
      { config, ... }:
      {
        programs.lazygit = {
          enable = true;
          enableZshIntegration = config.programs.zsh.enable;
          shellWrapperName = "lg";
          settings = {
            gui = {
              nerdFontsVersion = 3;
            };
          };
        };
      };
  };
}
