/*
  eza — modern ls replacement with icons, color, Git status, and tree view
  https://github.com/eza-community/eza
*/
_: {
  den.aspects.cli._.eza = {
    aspect-name = "cli.eza";
    homeManager =
      { config, ... }:
      {
        programs.eza = {
          enable = true;
          enableZshIntegration = config.programs.zsh.enable;
          icons = "auto";
          git = true;
        };

        programs.zsh.shellAliases = {
          ls = "eza --icons";
          ll = "eza -l --icons --git -a";
          lt = "eza --tree --level=2 --icons";
        };
      };
  };
}
