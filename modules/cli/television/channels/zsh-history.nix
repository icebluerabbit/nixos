{ den, ... }:
{
  den.aspects.cli._.television._.zsh-history = {
    aspect-name = "cli.television.zsh-history";
    includes = [ den.aspects.cli._.zsh ];

    homeManager =
      { pkgs, lib, ... }:
      let
        sed = lib.getExe' pkgs.gnused "sed";
        zsh = lib.getExe pkgs.zsh;
      in
      {
        programs.television.channels.zsh-history = {
          metadata = {
            name = "zsh-history";
            description = "A channel to select from your zsh history";
          };
          source = {
            command = "${sed} '1!G;h;$!d' \${HISTFILE:-\${HOME}/.zsh_history}";
            display = "{split:;:1..}";
            output = "{split:;:1..}";
            no_sort = true;
            frecency = false;
          };
          actions = {
            execute = {
              description = "Execute the selected command";
              command = "${zsh} -c '{split:;:1..}'";
              mode = "execute";
            };
          };
        };
      };
  };
}
