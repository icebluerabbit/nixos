{ den, ... }:
{
  den.aspects.cli._.television._.zoxide = {
    aspect-name = "cli.television.zoxide";
    includes = [ den.aspects.cli._.zoxide ];

    homeManager =
      { pkgs, lib, ... }:
      let
        zoxide = lib.getExe pkgs.zoxide;
        ls = lib.getExe' pkgs.coreutils "ls";
      in
      {
        programs.television.channels.zoxide = {
          metadata = {
            name = "zoxide";
            description = "Browse zoxide directory history";
          };
          source = {
            command = "${zoxide} query -l";
            no_sort = true;
            frecency = false;
          };
          preview = {
            command = "${ls} -la --color=always '{}'";
          };
          keybindings = {
            enter = "actions:cd";
            ctrl-d = "actions:remove";
          };
          actions = {
            cd = {
              description = "Change to the selected directory";
              command = "cd '{}' && $SHELL";
              mode = "execute";
            };
            remove = {
              description = "Remove the selected directory from zoxide";
              command = "${zoxide} remove '{}'";
              mode = "fork";
            };
          };
        };
      };
  };
}
