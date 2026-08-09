/*
  direnv — shell environment loader for per-directory .envrc files; nix-direnv caches nix develop shells
  https://github.com/direnv/direnv
*/
_: {
  den.aspects.cli._.direnv = {
    aspect-name = "cli.direnv";
    homeManager =
      { config, ... }:
      {
        programs.direnv = {
          enable = true;
          enableZshIntegration = config.programs.zsh.enable;
          nix-direnv.enable = true;
          # Prevent direnv shoowing all env variables on load
          config.global.hide_env_diff = true;
        };
      };
  };
}
