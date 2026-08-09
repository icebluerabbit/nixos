/*
  zoxide — smarter cd that tracks frecency (frequency + recency) and fuzzy-matches directory history
  https://github.com/ajeetdsouza/zoxide
*/
_: {
  den.aspects.cli._.zoxide = {
    aspect-name = "cli.zoxide";
    homeManager =
      { config, ... }:
      {
        programs.zoxide = {
          enable = true;
          enableZshIntegration = config.programs.zsh.enable;
          options = [ "--cmd cd" ];
        };

        # home-manager can't guarantee zoxide's init runs last in .zshrc (other
        # zsh hooks load after it), so zoxide's "doctor" prints a config-issue
        # warning on every shell/command init. The integration works fine —
        # silence the (benign) check.
        home.sessionVariables._ZO_DOCTOR = "0";
      };
  };
}
