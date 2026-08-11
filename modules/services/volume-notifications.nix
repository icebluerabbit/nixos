# volume-notifications — background service that listens for PipeWire volume changes and displays them via Mako
{ den, ... }:
{
  den.aspects.services._.volume-notifications = {
    aspect-name = "services.volume-notifications";
    includes = [
      den.aspects.gui._.mako
      den.aspects.hw._.audio
    ];

    homeManager =
      {
        pkgs,
        lib,
        ...
      }:
      {
        systemd.user.services.volume-notifications = {
          Unit = {
            Description = "Volume change notification listener";
            After = [
              "graphical-session.target"
              "mako.service"
            ];
            PartOf = [ "graphical-session.target" ];
          };
          Service = {
            ExecStart = lib.getExe pkgs.homelab.volume-notify;
            Restart = "always";
            RestartSec = 3;
          };
          Install.WantedBy = [ "graphical-session.target" ];
        };
      };
  };
}
