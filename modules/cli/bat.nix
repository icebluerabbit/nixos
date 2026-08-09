/*
  bat — cat clone with syntax highlighting and Git integration
  https://github.com/sharkdp/bat
*/
_: {
  den.aspects.cli._.bat = {
    aspect-name = "cli.bat";
    homeManager = _: {
      programs.bat = {
        enable = true;
        config = {
          #theme = "TwoDark";
        };
      };

      programs.zsh.shellAliases = {
        cat = "bat";
      };
    };
  };
}
