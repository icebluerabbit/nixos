/*
  Zsh — extended Bourne shell with programmable completion, history sharing, autosuggestions, and syntax highlighting
  https://github.com/zsh-users/zsh
*/
_: {
  den.aspects.cli._.zsh = {
    aspect-name = "cli.zsh";
    homeManager =
      { pkgs, ... }:
      {
        programs.zsh = {
          enable = true;
          enableCompletion = true;
          autosuggestion.enable = true;
          syntaxHighlighting.enable = true;
          history = {
            append = true;
            saveNoDups = true;
            ignoreDups = true;
            ignoreSpace = true;
            share = true;
            ignoreAllDups = true;
            findNoDups = true;
            size = 100000;
          };
          shellAliases = { };
          plugins = [
            {
              name = "zsh-nix-shell";
              file = "nix-shell.plugin.zsh";
              src = pkgs.fetchFromGitHub {
                owner = "chisui";
                repo = "zsh-nix-shell";
                rev = "v0.8.0";
                sha256 = "1lzrn0n4fxfcgg65v0qhnj7wnybybqzs4adz7xsrkgmcsr0ii8b7";
              };
            }
          ];
        };
      };
  };
}
