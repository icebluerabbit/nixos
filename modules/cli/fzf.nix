/*
  fzf — general-purpose fuzzy finder for files, history, and shell completions
  https://github.com/junegunn/fzf
*/
_: {
  den.aspects.cli._.fzf = {
    aspect-name = "cli.fzf";
    homeManager =
      { config, ... }:
      {
        programs.fzf = {
          enable = true;
          enableZshIntegration = config.programs.zsh.enable;
        };

        programs.zsh.shellAliases = { };
      };
  };
}
