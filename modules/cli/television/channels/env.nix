_: {
  den.aspects.cli._.television._.env = {
    aspect-name = "cli.television.env";
    includes = [ ];

    homeManager =
      { pkgs, lib, ... }:
      let
        printenv = lib.getExe' pkgs.coreutils "printenv";
      in
      {
        programs.television.channels.env = {
          metadata = {
            name = "env";
            description = "A channel to select from environment variables";
          };
          keybindings = {
            shortcut = "f3";
          };
          preview = {
            command = "echo '{split:=:1..}'";
          };
          source = {
            command = printenv;
            output = "{split:=:1..}";
          };
          ui = {
            layout = "portrait";
            preview_panel = {
              size = 20;
              header = "{split:=:0}";
            };
          };
          actions = {
            name = {
              description = "Output the variable name instead of the value";
              command = "echo '{split:=:0}'";
              mode = "execute";
            };
          };
        };
      };
  };
}
