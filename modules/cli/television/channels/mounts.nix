_: {
  den.aspects.cli._.television._.mounts = {
    aspect-name = "cli.television.mounts";
    includes = [ ];

    homeManager =
      { pkgs, lib, ... }:
      let
        df = lib.getExe' pkgs.coreutils "df";
        tail = lib.getExe' pkgs.coreutils "tail";
        head = lib.getExe' pkgs.coreutils "head";
        ls = lib.getExe' pkgs.coreutils "ls";
      in
      {
        programs.television.channels.mounts = {
          metadata = {
            name = "mounts";
            description = "List mounted filesystems";
          };
          keybindings = {
            enter = "actions:cd";
          };
          preview = {
            command = "${df} -h '{0}' && echo && ${ls} -la '{0}' 2>/dev/null | ${head} -20";
          };
          source = {
            command = "${df} -h --output=target,fstype,size,used,avail,pcent 2>/dev/null | ${tail} -n +2";
            display = "{split: :0}";
          };
          actions = {
            cd = {
              description = "Open a shell in the selected mount point";
              command = "cd '{}' && $SHELL";
              mode = "execute";
            };
          };
        };
      };
  };
}
