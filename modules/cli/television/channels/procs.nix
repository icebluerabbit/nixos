_: {
  den.aspects.cli._.television._.procs = {
    aspect-name = "cli.television.procs";
    includes = [ ];

    homeManager =
      { pkgs, lib, ... }:
      # Uses procps `ps` with Linux-specific flags; Linux-only.
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux (
        let
          ps = lib.getExe' pkgs.procps "ps";
          awk = lib.getExe pkgs.gawk;
          fold = lib.getExe' pkgs.coreutils "fold";
        in
        {
          programs.television.channels.procs = {
            metadata = {
              name = "procs";
              description = "A channel to find and manage running processes";
            };
            source = {
              command = "${ps} -e -o pid=,ucomm= | ${awk} '{print $1, $2}'";
              display = "{split: :1}";
              output = "{split: :0}";
            };
            preview = {
              command = "${ps} -p '{split: :0}' -o user,pid,ppid,state,%cpu,%mem,command | ${fold}";
            };
            keybindings = {
              ctrl-k = "actions:kill";
              f2 = "actions:term";
              ctrl-s = "actions:stop";
              ctrl-c = "actions:cont";
            };
            actions = {
              kill = {
                description = "Kill the selected process (SIGKILL)";
                command = "kill -9 {split: :0}";
                mode = "execute";
              };
              term = {
                description = "Terminate the selected process (SIGTERM)";
                command = "kill -15 {split: :0}";
                mode = "execute";
              };
              stop = {
                description = "Stop/pause the selected process (SIGSTOP)";
                command = "kill -STOP {split: :0}";
                mode = "fork";
              };
              cont = {
                description = "Continue/resume the selected process (SIGCONT)";
                command = "kill -CONT {split: :0}";
                mode = "fork";
              };
            };
          };
        }
      );
  };
}
