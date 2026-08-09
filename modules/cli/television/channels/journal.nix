{ den, ... }:
{
  den.aspects.cli._.television._.journal = {
    aspect-name = "cli.television.journal";
    includes = [
      den.aspects.cli._.git
    ];

    homeManager =
      { pkgs, lib, ... }:
      # Browses the systemd journal via journalctl; Linux-only.
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux (
        let
          ctl = lib.getExe' pkgs.systemd "journalctl";
          coresort = lib.getExe' pkgs.coreutils "sort";
          less = lib.getExe pkgs.less;
        in
        {
          programs.television.channels.journal = {
            metadata = {
              name = "journal";
              description = "Browse systemd journal log identifiers and their logs";
            };
            preview = {
              command = "${ctl} -b --no-pager -o short-iso -n 50 SYSLOG_IDENTIFIER='{0}' 2>/dev/null";
            };
            source = {
              command = "${ctl} --field SYSLOG_IDENTIFIER 2>/dev/null | ${coresort} -f";
            };
            ui = {
              layout = "portrait";
              preview_panel = {
                size = 70;
              };
            };
            actions = {
              logs = {
                description = "Follow live logs for the selected identifier";
                command = "${ctl} -f SYSLOG_IDENTIFIER='{0}'";
                mode = "execute";
              };
              full = {
                description = "View all logs for the selected identifier in a pager";
                command = "${ctl} -b --no-pager -o short-iso SYSLOG_IDENTIFIER='{0}' | ${less}";
                mode = "fork";
              };
            };
          };
        }
      );
  };
}
