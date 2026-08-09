/*
  ripgrep — line-oriented regex search; respects .gitignore, faster than grep/ag
  https://github.com/BurntSushi/ripgrep
*/
_: {
  den.aspects.cli._.ripgrep = {
    aspect-name = "cli.ripgrep";
    homeManager = {
      programs.ripgrep = {
        enable = true;
        arguments = [ "--smart-case" ];
      };

      programs.zsh.shellAliases = {
        grep = "rg";
      };
    };
  };
}
