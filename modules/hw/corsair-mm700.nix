/*
  corsair-mm700 — Corsair MM700 RGB mousepad color customization via OpenRGB
  https://gitlab.com/CalcProgrammer1/OpenRGB
*/
{ den, ... }:
{
  den.aspects.hw._.corsair-mm700 = {
    aspect-name = "hw.corsair-mm700";
    includes = [ den.aspects.hw._.rgb ];
    nixos =
      { pkgs, lib, ... }:
      {
        systemd.services.corsair-mm700-rgb = {
          wantedBy = [ "multi-user.target" ];
          after = [ "openrgb.service" ];
          requires = [ "openrgb.service" ];
          serviceConfig = {
            Type = "oneshot";
            RemainAfterExit = true;
            # openrgb.service is "started" once its socket is up, but I2C device
            # enumeration takes a few more seconds — --retries gives the mousepad
            # ~30s to appear so cold boot doesn't race.
            ExecStart = lib.escapeShellArgs [
              (lib.getExe pkgs.homelab.openrgb-init)
              "--openrgb"
              (lib.getExe pkgs.openrgb)
              "--device"
              "Corsair MM700"
              "--retries"
              "30"
              "--set"
              "0:ffffff,ffffff,00ff00"
            ];
          };
        };
      };
  };
}
